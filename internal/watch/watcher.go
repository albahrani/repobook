package watch

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"

	"repobook/internal/util"
)

type Watcher struct {
	rootAbs string
	hub     *Hub
	w       *fsnotify.Watcher
	done    chan struct{}
}

func NewWatcher(rootAbs string, hub *Hub) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ww := &Watcher{rootAbs: rootAbs, hub: hub, w: w, done: make(chan struct{})}

	// Watch all directories initially (fsnotify is not recursive).
	err = filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return fs.SkipDir
			}
			return w.Add(p)
		}
		return nil
	})
	if err != nil {
		_ = w.Close()
		return nil, err
	}

	go ww.loop()
	return ww, nil
}

func (w *Watcher) Close() error {
	close(w.done)
	return w.w.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case <-w.w.Errors:
			// ignore
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	// If a new directory appears, start watching it.
	if ev.Op&fsnotify.Create != 0 {
		if st, err := util.Stat(ev.Name); err == nil && st.IsDir() {
			_ = w.w.Add(ev.Name)
			w.hub.Broadcast(Event{Type: "tree-updated"})
			return
		}
	}

	relOS, err := filepath.Rel(w.rootAbs, ev.Name)
	if err != nil {
		return
	}
	rel := filepath.ToSlash(relOS)
	name := filepath.Base(ev.Name)
	if util.IsMarkdownFileName(name) {
		w.hub.Broadcast(Event{Type: "file-changed", Path: rel})
		if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
			w.hub.Broadcast(Event{Type: "tree-updated"})
		}
		return
	}

	// If a markdown file was removed/renamed, tree may change.
	if strings.HasSuffix(strings.ToLower(name), ".md") && ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		w.hub.Broadcast(Event{Type: "tree-updated"})
	}
}
