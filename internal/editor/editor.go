package editor

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/outsideris/retrotech/internal/builder"
	"github.com/outsideris/retrotech/internal/parser"
)

// assetsFS holds the editor's front-end (index.html, app.js, app.css), embedded
// into the binary so the desktop app ships as a single self-contained server —
// no separate front-end files to locate at runtime.
//
//go:embed assets
var assetsFS embed.FS

// Config configures the editor server.
type Config struct {
	// RepoDir is the RetroTech project root. The editor reads/writes episodes
	// under content/episodes and serves public/ for preview assets.
	RepoDir string
}

// Editor serves the episode management UI, its JSON API, and HTML previews.
type Editor struct {
	store     *Store
	publicDir string
	assets    fs.FS
	year      int
}

// New builds an Editor for the given repo. It fails if content/episodes is
// missing, so the server never silently points at the wrong folder.
func New(cfg Config) (*Editor, error) {
	episodesDir := filepath.Join(cfg.RepoDir, "content", "episodes")
	info, err := os.Stat(episodesDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%q doesn't look like the RetroTech repo (missing content/episodes)", cfg.RepoDir)
	}
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, err
	}
	return &Editor{
		store:     NewStore(episodesDir),
		publicDir: filepath.Join(cfg.RepoDir, "public"),
		assets:    sub,
		year:      time.Now().Year(),
	}, nil
}

// Handler returns the HTTP handler for the editor:
//
//	GET    /_write/                  UI shell + embedded assets
//	GET    /_write/api/episodes      list
//	POST   /_write/api/episodes      create
//	GET    /_write/api/episodes/{id} load one
//	PUT    /_write/api/episodes/{id} update
//	DELETE /_write/api/episodes/{id} delete
//	POST   /_write/api/preview       render an episode page (no save)
//	/                                redirect to /_write/, else serve public/
func (e *Editor) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_write/api/episodes", e.handleList)
	mux.HandleFunc("POST /_write/api/episodes", e.handleCreate)
	mux.HandleFunc("GET /_write/api/episodes/{id}", e.handleGet)
	mux.HandleFunc("PUT /_write/api/episodes/{id}", e.handleUpdate)
	mux.HandleFunc("DELETE /_write/api/episodes/{id}", e.handleDelete)
	mux.HandleFunc("POST /_write/api/preview", e.handlePreview)
	mux.Handle("GET /_write/", http.StripPrefix("/_write", http.FileServerFS(e.assets)))
	mux.HandleFunc("/", e.handleRoot)
	return mux
}

func (e *Editor) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := e.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (e *Editor) handleGet(w http.ResponseWriter, r *http.Request) {
	form, err := e.store.Get(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, form)
}

func (e *Editor) handleCreate(w http.ResponseWriter, r *http.Request) {
	form, err := decodeForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := e.store.Create(form); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, form)
}

func (e *Editor) handleUpdate(w http.ResponseWriter, r *http.Request) {
	form, err := decodeForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := e.store.Update(r.PathValue("id"), form); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, form)
}

func (e *Editor) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := e.store.Delete(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePreview renders the live episode page for the posted form without
// saving it, reusing the production builder so the preview matches the built
// site. Its asset references (/styles.css, /badges/*, /images/*) resolve
// against this server's public/ handler.
func (e *Editor) handlePreview(w http.ResponseWriter, r *http.Request) {
	form, err := decodeForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ep := formToEpisode(form)
	if ep.ID == "" {
		ep.ID = "preview"
	}
	html := builder.BuildEpisodePage(ep, builder.Site{Year: e.year})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleRoot redirects "/" to the editor and otherwise serves files from
// public/ (the assets a preview page links to).
func (e *Editor) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/_write/", http.StatusFound)
		return
	}
	http.FileServer(http.Dir(e.publicDir)).ServeHTTP(w, r)
}

// formToEpisode rebuilds a parser.Episode from a form for rendering. It reuses
// composeBody so the preview renders the exact body that would be written.
func formToEpisode(f EpisodeForm) parser.Episode {
	return parser.Episode{
		Frontmatter: parser.Frontmatter{
			Title:        f.Title,
			Date:         f.Date,
			Description:  f.Description,
			Description2: f.Description2,
			Author:       f.Author,
			Enclosure:    parser.Enclosure{URL: f.EnclosureURL, Size: f.EnclosureSize},
			Duration:     f.Duration,
			Badges:       f.Badges,
		},
		ID:   f.ID,
		Body: composeBody(f),
	}
}

func decodeForm(r *http.Request) (EpisodeForm, error) {
	var f EpisodeForm
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return EpisodeForm{}, fmt.Errorf("invalid request body: %w", err)
	}
	return f, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeStoreError maps store errors to HTTP status codes.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
