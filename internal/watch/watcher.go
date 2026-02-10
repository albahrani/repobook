package watch

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"

	"repobook/internal/ignore"
	"repobook/internal/util"
)

type Watcher struct {
	rootAbs string
	ignore  *ignore.Matcher
	hub     *Hub
	w       *fsnotify.Watcher
	done    chan struct{}
}

func NewWatcher(rootAbs string, hub *Hub, ig *ignore.Matcher) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ww := &Watcher{rootAbs: rootAbs, ignore: ig, hub: hub, w: w, done: make(chan struct{})}

	// Start the event loop before adding watches to prevent deadlock on Windows
	// where fsnotify may send events synchronously during Add()
	go ww.loop()

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

			relOS, err := filepath.Rel(rootAbs, p)
			if err != nil {
				return nil
			}
			rel := filepath.ToSlash(relOS)
			if rel == "." {
				rel = ""
			}
			if ww.ignore != nil && rel != "" && ww.ignore.IsIgnored(rel, true) {
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
			relOS, err := filepath.Rel(w.rootAbs, ev.Name)
			if err == nil {
				rel := filepath.ToSlash(relOS)
				if rel == "." {
					rel = ""
				}
				if w.ignore != nil && rel != "" && w.ignore.IsIgnored(rel, true) {
					return
				}
			}
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
	if rel == "." {
		rel = ""
	}
	if w.ignore != nil && rel != "" {
		if st, err := util.Stat(ev.Name); err == nil {
			if w.ignore.IsIgnored(rel, st.IsDir()) {
				return
			}
		}
	}
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
