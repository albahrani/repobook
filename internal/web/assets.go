package web

import (
	"embed"
	"io"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

func FS() http.FileSystem {
	sub, _ := fsSub(staticFS, "static")
	return http.FS(sub)
}

func ServeIndex(w http.ResponseWriter, r *http.Request) {
	f, err := staticFS.Open("static/index.html")
	if err != nil {
		http.Error(w, "missing index", http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.Copy(w, f)
}
