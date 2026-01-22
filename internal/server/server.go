package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"repobook/internal/ignore"
	"repobook/internal/render"
	"repobook/internal/scan"
	"repobook/internal/search"
	"repobook/internal/util"
	"repobook/internal/watch"
	"repobook/internal/web"
)

type Options struct {
	Root string
}

type Server struct {
	rootAbs  string
	ignore   *ignore.Matcher
	renderer *render.Renderer
	hub      *watch.Hub
	watcher  *watch.Watcher
}

func New(opts Options) (*Server, error) {
	rootAbs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	ig, err := ignore.Load(rootAbs)
	if err != nil {
		return nil, err
	}

	hub := watch.NewHub()
	w, err := watch.NewWatcher(rootAbs, hub, ig)
	if err != nil {
		return nil, err
	}

	r, err := render.New(render.Options{RepoRootAbs: rootAbs})
	if err != nil {
		_ = w.Close()
		return nil, err
	}

	s := &Server{
		rootAbs:  rootAbs,
		ignore:   ig,
		renderer: r,
		hub:      hub,
		watcher:  w,
	}

	return s, nil
}

func (s *Server) Close() error {
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// App assets (embedded)
	mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(web.FS())))

	// Repo assets (served from the target directory, read-only)
	mux.HandleFunc("/repo/", s.handleRepoAsset)

	// API
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/home", s.handleHome)
	mux.HandleFunc("/api/render", s.handleRender)
	mux.HandleFunc("/api/search", s.handleSearch)

	// WebSocket
	mux.HandleFunc("/ws", s.hub.ServeWS)

	// Client routes
	mux.HandleFunc("/file/", s.handleIndex)
	mux.HandleFunc("/", s.handleIndex)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	web.ServeIndex(w, r)
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tree, err := scan.BuildTree(scan.Options{RootAbs: s.rootAbs, Ignore: s.ignore})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, tree)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel, err := util.ResolveDefaultReadmeRel(s.rootAbs)
	if err != nil {
		// If there is no README at root, still return something predictable.
		rel = ""
	}

	writeJSON(w, map[string]string{"path": rel})
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("path")
	if q == "" {
		// Default is README.md at repo root.
		rel, err := util.ResolveDefaultReadmeRel(s.rootAbs)
		if err != nil {
			http.Error(w, "no README.md found at repo root", http.StatusNotFound)
			return
		}
		q = rel
	}

	// Accept either URL-escaped or raw.
	if unesc, err := url.PathUnescape(q); err == nil {
		q = unesc
	}

	resolved, err := util.ResolveMarkdownRel(s.rootAbs, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if s.ignore != nil && resolved.Rel != "" && s.ignore.IsIgnored(resolved.Rel, false) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	res, err := s.renderer.RenderFile(resolved.Rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, res)
}

func (s *Server) handleRepoAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// URL paths are always forward slashes.
	relURL := strings.TrimPrefix(r.URL.Path, "/repo/")
	relURL, _ = url.PathUnescape(relURL)
	relURL = path.Clean("/" + relURL)
	relURL = strings.TrimPrefix(relURL, "/")

	abs, _, err := util.ResolveRepoPath(s.rootAbs, relURL)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if s.ignore != nil && relURL != "" && s.ignore.IsIgnored(relURL, false) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Prevent directory listings.
	if st, err := util.Stat(abs); err == nil && st.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Light caching; live reload will refresh content anyway.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, abs)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	res, err := search.Ripgrep(s.rootAbs, q, 200)
	if err != nil {
		if err == search.ErrRipgrepNotFound {
			http.Error(w, "ripgrep (rg) not found on PATH", http.StatusNotImplemented)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, res)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
