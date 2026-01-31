package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

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

	// RepoAssetHost/RepoAssetPort control the separate server that serves raw
	// repo files (images, PDFs, etc). This keeps untrusted repo content on a
	// different origin than the app UI and its API.
	//
	// If RepoAssetPort is 0, an available port is chosen.
	RepoAssetHost string
	RepoAssetPort int
}

type Server struct {
	rootAbs  string
	ignore   *ignore.Matcher
	renderer *render.Renderer
	hub      *watch.Hub
	watcher  *watch.Watcher

	repoAssetBaseURL string
	repoAssetSrv     *http.Server
	repoAssetLn      net.Listener
}

func New(opts Options) (*Server, error) {
	rootAbs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	if opts.RepoAssetHost == "" {
		opts.RepoAssetHost = "127.0.0.1"
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

	// Serve repo assets from a different origin than the app UI.
	// This prevents raw HTML/JS inside the repo from becoming same-origin with
	// the repobook UI + API.
	if err := s.startRepoAssetServer(opts.RepoAssetHost, opts.RepoAssetPort); err != nil {
		_ = w.Close()
		return nil, err
	}

	return s, nil
}

func (s *Server) Close() error {
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	if s.repoAssetSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.repoAssetSrv.Shutdown(ctx)
	}
	if s.repoAssetLn != nil {
		_ = s.repoAssetLn.Close()
	}
	return nil
}

func (s *Server) RepoAssetBaseURL() string {
	return s.repoAssetBaseURL
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// App assets (embedded)
	mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(web.FS())))

	// Repo assets (served from the target directory, read-only)
	// Note: this handler redirects to a separate repo-asset server running on a
	// different origin.
	mux.HandleFunc("/repo/", s.handleRepoAssetRedirect)

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

func (s *Server) startRepoAssetServer(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.repoAssetLn = ln
	base := fmt.Sprintf("http://%s/", ln.Addr().String())
	s.repoAssetBaseURL = base

	assetMux := http.NewServeMux()
	assetMux.HandleFunc("/", s.handleRepoAssetDirect)

	srv := &http.Server{
		Handler:      assetMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	s.repoAssetSrv = srv

	go func() {
		_ = srv.Serve(ln)
	}()

	return nil
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

func (s *Server) handleRepoAssetRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// URL paths are always forward slashes.
	relURL := strings.TrimPrefix(r.URL.Path, "/repo/")
	relURL, _ = url.PathUnescape(relURL)
	relURL = path.Clean("/" + relURL)
	relURL = strings.TrimPrefix(relURL, "/")
	if relURL == "." {
		relURL = ""
	}

	if s.repoAssetLn == nil || s.repoAssetBaseURL == "" {
		http.Error(w, "repo asset server not available", http.StatusServiceUnavailable)
		return
	}

	u, _ := url.Parse(s.repoAssetBaseURL)
	u.Path = "/" + relURL
	u.RawQuery = r.URL.RawQuery
	// Permanent redirect is safe since the chosen port is stable for the process.
	http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
}

func (s *Server) handleRepoAssetDirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// URL paths are always forward slashes.
	// On the asset server we serve files from the repo root at "/".
	relURL := strings.TrimPrefix(r.URL.Path, "/")
	relURL, _ = url.PathUnescape(relURL)
	relURL = path.Clean("/" + relURL)
	relURL = strings.TrimPrefix(relURL, "/")
	if relURL == "." {
		relURL = ""
	}

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
	// Defensive defaults.
	w.Header().Set("X-Content-Type-Options", "nosniff")
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
