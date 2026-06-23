package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DraftStore manages in-progress episodes as JSON files under content/drafts.
//
// Drafts are kept separate from published episodes (content/episodes) on
// purpose: the site build and the RSS feed only read content/episodes, so a
// draft is invisible until it is published. JSON (rather than episode markdown)
// is used because a draft holds the *whole* editable form — including the
// intended episode id and any half-filled fields — which has no clean home in
// episode frontmatter (where the id is the filename, not a field). Publishing
// composes the form into content/episodes/<id>.md.
type DraftStore struct {
	dir string
	now func() time.Time // injectable so tests get deterministic slugs/timestamps
}

// NewDraftStore returns a DraftStore backed by the given drafts directory.
func NewDraftStore(dir string) *DraftStore {
	return &DraftStore{dir: dir, now: time.Now}
}

// DraftSummary is a sidebar entry for a draft.
type DraftSummary struct {
	Slug    string `json:"slug"`    // the draft's own id (draft-YYYYMMDD-HHMMSS)
	Title   string `json:"title"`   // form title (may be empty)
	ID      string `json:"id"`      // intended episode id (may be empty)
	Updated string `json:"updated"` // RFC3339, last save
}

// draftFile is the on-disk JSON shape.
type draftFile struct {
	Form    EpisodeForm `json:"form"`
	Updated string      `json:"updated"`
}

func (ds *DraftStore) path(slug string) (string, error) {
	if !ValidID(slug) {
		return "", ErrInvalidID
	}
	return filepath.Join(ds.dir, slug+".json"), nil
}

// Create makes a new empty draft (today's date, default author) and returns its
// slug and starting form. The slug is timestamp-based, with a numeric suffix on
// the rare same-second collision so a quick double-click can't clobber a draft.
func (ds *DraftStore) Create() (string, EpisodeForm, error) {
	if err := os.MkdirAll(ds.dir, 0755); err != nil {
		return "", EpisodeForm{}, err
	}
	base := "draft-" + ds.now().Format("20060102-150405")
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(ds.dir, slug+".json")); errors.Is(err, os.ErrNotExist) {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	form := EpisodeForm{
		Date:       ds.now().Format("2006/01/02"),
		Author:     "Outsider",
		Structured: true,
	}
	if err := ds.write(slug, form); err != nil {
		return "", EpisodeForm{}, err
	}
	return slug, form, nil
}

func (ds *DraftStore) write(slug string, form EpisodeForm) error {
	p, err := ds.path(slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ds.dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(draftFile{Form: form, Updated: ds.now().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(p, data)
}

func (ds *DraftStore) read(slug string) (draftFile, error) {
	p, err := ds.path(slug)
	if err != nil {
		return draftFile{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return draftFile{}, ErrNotFound
		}
		return draftFile{}, err
	}
	var df draftFile
	if err := json.Unmarshal(data, &df); err != nil {
		return draftFile{}, fmt.Errorf("corrupt draft %s: %w", slug, err)
	}
	return df, nil
}

// List returns every draft as a summary, most-recently-updated first. A missing
// drafts directory is simply an empty list.
func (ds *DraftStore) List() ([]DraftSummary, error) {
	entries, err := os.ReadDir(ds.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []DraftSummary{}, nil
		}
		return nil, err
	}
	out := []DraftSummary{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".json")
		df, err := ds.read(slug)
		if err != nil {
			continue // skip unreadable/corrupt drafts rather than failing the list
		}
		out = append(out, DraftSummary{
			Slug:    slug,
			Title:   strings.TrimSpace(df.Form.Title),
			ID:      df.Form.ID,
			Updated: df.Updated,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out, nil
}

// Get loads a draft's form.
func (ds *DraftStore) Get(slug string) (EpisodeForm, error) {
	df, err := ds.read(slug)
	if err != nil {
		return EpisodeForm{}, err
	}
	return df.Form, nil
}

// Save overwrites an existing draft (the auto-save path). A missing draft is
// ErrNotFound so a stale tab can't resurrect a published/deleted draft.
func (ds *DraftStore) Save(slug string, form EpisodeForm) error {
	if _, err := ds.read(slug); err != nil {
		return err
	}
	return ds.write(slug, form)
}

// Delete removes a draft.
func (ds *DraftStore) Delete(slug string) error {
	p, err := ds.path(slug)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Publish promotes a draft to a published episode: it writes
// content/episodes/<id>.md from the draft's form (via the episode store, which
// validates the id and refuses to overwrite an existing episode) and then
// removes the draft. The episode id comes from the form, so the author must set
// a valid, unused id before publishing.
func (ds *DraftStore) Publish(slug string, episodes *Store) (string, error) {
	form, err := ds.Get(slug)
	if err != nil {
		return "", err
	}
	if err := episodes.Create(form); err != nil {
		return "", err // ErrInvalidID / ErrExists surface to the UI
	}
	// The episode is written; removing the draft is best-effort cleanup.
	_ = ds.Delete(slug)
	return form.ID, nil
}
