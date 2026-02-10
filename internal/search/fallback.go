package search

import (
	"bufio"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"repobook/internal/ignore"
	"repobook/internal/util"
)

// Fallback performs a best-effort fixed-string search without relying on ripgrep.
// It scans markdown files under rootAbs, respecting .gitignore (best-effort) and
// common heavyweight directories.
//
// It is intentionally simple: fixed-string match with smart-case, returns up to
// limit results, and stops after a small time budget.
func Fallback(rootAbs string, ig *ignore.Matcher, query string, limit int) (Response, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Response{Query: query, Results: nil, Truncated: false}, nil
	}
	if limit <= 0 {
		limit = 200
	}

	deadline := time.Now().Add(3 * time.Second)
	resp := Response{Query: query, Results: make([]Result, 0, 32)}

	ignoreDirs := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"vendor":       {},
		".idea":        {},
		".vscode":      {},
	}

	caseSensitive := false
	for _, r := range query {
		if 'A' <= r && r <= 'Z' {
			caseSensitive = true
			break
		}
	}

	qLower := ""
	if !caseSensitive {
		qLower = strings.ToLower(query)
	}

	err := filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Best-effort; ignore unreadable entries.
			return nil
		}
		if time.Now().After(deadline) {
			resp.Truncated = true
			return fs.SkipAll
		}

		relOS, err := filepath.Rel(rootAbs, p)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(relOS)
		if rel == "." {
			rel = ""
		}

		if d.IsDir() {
			if _, ok := ignoreDirs[d.Name()]; ok {
				return fs.SkipDir
			}
			if ig != nil && rel != "" && ig.IsIgnored(rel, true) {
				return fs.SkipDir
			}
			return nil
		}

		if rel != "" && ig != nil && ig.IsIgnored(rel, false) {
			return nil
		}

		if !util.IsMarkdownFileName(d.Name()) {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		// Allow larger lines than the default 64K.
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNo := 0
		for s.Scan() {
			lineNo++
			if time.Now().After(deadline) {
				resp.Truncated = true
				return fs.SkipAll
			}
			line := strings.TrimRight(s.Text(), "\r\n")
			if line == "" {
				continue
			}
			matched := false
			if caseSensitive {
				matched = strings.Contains(line, query)
			} else {
				matched = strings.Contains(strings.ToLower(line), qLower)
			}
			if !matched {
				continue
			}

			resp.Results = append(resp.Results, Result{Path: path.Clean(rel), Line: lineNo, Preview: line})
			if len(resp.Results) >= limit {
				resp.Truncated = true
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, err
	}

	return resp, nil
}
