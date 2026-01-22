package util

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func IsMarkdownFileName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func LooksLikeMarkdownPath(rel string) bool {
	// Treat folders as markdown targets too (README.md resolution on the server).
	if rel == "" {
		return true
	}
	lower := strings.ToLower(rel)
	if strings.HasSuffix(lower, "/") {
		return true
	}
	if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown") {
		return true
	}
	// Common pattern: links to folder without trailing slash.
	if !strings.Contains(path.Base(rel), ".") {
		return true
	}
	return false
}

type Resolved struct {
	Abs string
	Rel string // forward slashes
}

func ResolveRepoPath(rootAbs, relURL string) (abs string, rel string, err error) {
	relURL = strings.TrimPrefix(relURL, "/")
	relURL = path.Clean("/" + relURL)
	relURL = strings.TrimPrefix(relURL, "/")
	if relURL == "." {
		relURL = ""
	}
	relOS := filepath.FromSlash(relURL)
	abs = filepath.Join(rootAbs, relOS)

	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", "", err
	}
	rootAbs2, err := filepath.Abs(rootAbs)
	if err != nil {
		return "", "", err
	}

	// Ensure the resolved path stays within root.
	relCheck, err := filepath.Rel(rootAbs2, abs)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(relCheck, "..") || relCheck == ".." {
		return "", "", errors.New("path escapes repo root")
	}
	return abs, filepath.ToSlash(relCheck), nil
}

func ResolveDefaultReadmeRel(rootAbs string) (string, error) {
	// Case-insensitive search. We avoid scanning the whole tree.
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), "README.md") {
			return e.Name(), nil
		}
	}
	return "", os.ErrNotExist
}

func ResolveMarkdownRel(rootAbs, rel string) (Resolved, error) {
	rel = filepath.ToSlash(rel)
	abs, cleanRel, err := ResolveRepoPath(rootAbs, rel)
	if err != nil {
		return Resolved{}, err
	}

	st, statErr := os.Stat(abs)
	if statErr != nil {
		return Resolved{}, statErr
	}
	if st.IsDir() {
		// Directory default is README.md
		entries, err := os.ReadDir(abs)
		if err != nil {
			return Resolved{}, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.EqualFold(e.Name(), "README.md") {
				rr := path.Join(cleanRel, e.Name())
				aa, _, err := ResolveRepoPath(rootAbs, rr)
				if err != nil {
					return Resolved{}, err
				}
				return Resolved{Abs: aa, Rel: rr}, nil
			}
		}
		return Resolved{}, os.ErrNotExist
	}

	if !IsMarkdownFileName(path.Base(cleanRel)) {
		return Resolved{}, errors.New("not a markdown file")
	}
	return Resolved{Abs: abs, Rel: cleanRel}, nil
}

func Stat(abs string) (os.FileInfo, error) {
	return os.Stat(abs)
}
