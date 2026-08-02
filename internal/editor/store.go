package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/outsideris/retrotech/internal/parser"
)

// Store reads and writes episode markdown files in a single directory
// (content/episodes). There is no index or database: the set of *.md files is
// the set of episodes, so create/rename/delete are plain file operations and
// the site build re-derives everything from disk.
type Store struct {
	dir string
}

// NewStore returns a Store backed by the given episodes directory.
func NewStore(episodesDir string) *Store {
	return &Store{dir: episodesDir}
}

// Store errors. Handlers map these to HTTP status codes.
var (
	ErrInvalidID = errors.New("invalid episode id")
	ErrNotFound  = errors.New("episode not found")
	ErrExists    = errors.New("episode already exists")
)

// idPattern constrains episode ids to a safe slug. The id is the filename, the
// URL path and the feed guid, so it must be filesystem- and URL-clean; the
// pattern also blocks path traversal (no "/", no "..", no leading dot).
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidID reports whether id is an acceptable episode slug.
func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

// path returns the file path for id, rejecting unsafe ids before any
// filesystem access.
func (s *Store) path(id string) (string, error) {
	if !ValidID(id) {
		return "", ErrInvalidID
	}
	return filepath.Join(s.dir, id+".md"), nil
}

// List returns every episode as a summary, newest-first (parser.LoadEpisodes
// applies the same ordering the site build and feed use).
func (s *Store) List() ([]EpisodeSummary, error) {
	eps, err := parser.LoadEpisodes(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]EpisodeSummary, 0, len(eps))
	for _, ep := range eps {
		out = append(out, summaryOf(ep))
	}
	return out, nil
}

// Get loads one episode as an editable form. A missing file is ErrNotFound.
func (s *Store) Get(id string) (EpisodeForm, error) {
	p, err := s.path(id)
	if err != nil {
		return EpisodeForm{}, err
	}
	ep, err := parser.LoadEpisode(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EpisodeForm{}, ErrNotFound
		}
		return EpisodeForm{}, err
	}
	return EpisodeToForm(ep), nil
}

// Create writes a new episode. It fails with ErrExists if the id is taken, so a
// create never clobbers an existing episode. For a structured form the
// description is derived from the intro (see derive.go), never taken from the
// caller.
func (s *Store) Create(f EpisodeForm) error {
	p, err := s.path(f.ID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return ErrExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if f.Structured {
		f.Description = deriveDescription(f.Intro)
	}
	return writeAtomic(p, ComposeFile(f))
}

// Update overwrites an existing episode in place. The id is taken from the path,
// not the body, so the URL/guid can't change by accident; a missing file is
// ErrNotFound.
//
// For a structured form the description follows the intro, not the caller: an
// unchanged intro keeps the stored description byte-exact (legacy episodes wrap
// their description differently from their intro, and rewriting it would churn
// the RSS feed on an unrelated edit), while a changed intro re-derives it.
func (s *Store) Update(id string, f EpisodeForm) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	f.ID = id
	if f.Structured {
		if cur, err := s.Get(id); err == nil && cur.Structured && cur.Intro == f.Intro {
			f.Description = cur.Description
		} else {
			f.Description = deriveDescription(f.Intro)
		}
	}
	return writeAtomic(p, ComposeFile(f))
}

// Delete removes an episode file. A missing file is ErrNotFound.
func (s *Store) Delete(id string) error {
	p, err := s.path(id)
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

// writeAtomic writes data to a temp file in the destination directory and
// renames it into place, so a crash mid-write can't leave a half-written
// episode file (the build would fail to parse it).
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".episode-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
