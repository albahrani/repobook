package render

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"repobook/internal/util"
)

var linkCtxKeyCurrentRel = parser.NewContextKey()

type linkRewriter struct {
	repoRootAbs string
	ignore      interface {
		IsIgnored(relSlash string, isDir bool) bool
	}
}

func (t *linkRewriter) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	curRel, _ := pc.Get(linkCtxKeyCurrentRel).(string)
	curDir := path.Dir(filepath.ToSlash(curRel))
	if curDir == "." {
		curDir = ""
	}

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch v := n.(type) {
		case *ast.Link:
			dest, openNewTab, missing := t.rewriteURLDest(curDir, v.Destination)
			v.Destination = dest
			if missing {
				if cls, ok := v.AttributeString("class"); ok {
					if b, ok := cls.([]byte); ok && len(b) > 0 {
						v.SetAttributeString("class", append(append([]byte{}, b...), []byte(" repobook-missing")...))
					} else if s, ok := cls.(string); ok && strings.TrimSpace(s) != "" {
						v.SetAttributeString("class", []byte(strings.TrimSpace(s)+" repobook-missing"))
					} else {
						v.SetAttributeString("class", []byte("repobook-missing"))
					}
				} else {
					v.SetAttributeString("class", []byte("repobook-missing"))
				}
				if _, ok := v.AttributeString("title"); !ok {
					v.SetAttributeString("title", []byte("Target not found in repo"))
				}
				v.SetAttributeString("data-repobook-missing", []byte("1"))
			}
			if openNewTab {
				// For repo assets and external HTTP(S) links, open in a new tab.
				v.SetAttributeString("target", []byte("_blank"))
				v.SetAttributeString("rel", []byte("noopener noreferrer"))
			}
		case *ast.AutoLink:
			_, openNewTab, _ := t.rewriteURLDest(curDir, v.URL(reader.Source()))
			if openNewTab {
				v.SetAttributeString("target", []byte("_blank"))
				v.SetAttributeString("rel", []byte("noopener noreferrer"))
			}
		case *ast.Image:
			dest, _, missing := t.rewriteURLDest(curDir, v.Destination)
			v.Destination = dest
			if missing {
				v.SetAttributeString("data-repobook-missing", []byte("1"))
			}
		}
		return ast.WalkContinue, nil
	})
}

func (t *linkRewriter) rewriteURLDest(curDir string, dest []byte) (_ []byte, openNewTab bool, missing bool) {
	raw := strings.TrimSpace(string(dest))
	if raw == "" {
		return dest, false, false
	}
	if strings.HasPrefix(raw, "#") {
		return dest, false, false
	}

	u, err := url.Parse(raw)
	if err != nil {
		return dest, false, false
	}
	if u.Host != "" {
		// Scheme-relative links like //example.com.
		return dest, true, false
	}
	if u.Scheme != "" {
		// Only treat HTTP(S) as a browser-navigation link.
		if u.Scheme == "http" || u.Scheme == "https" {
			return dest, true, false
		}
		// Common external navigations/handlers.
		if u.Scheme == "mailto" || u.Scheme == "tel" {
			return dest, true, false
		}
		return dest, false, false
	}

	p := u.Path
	if p == "" {
		return dest, false, false
	}

	// Resolve relative to current file dir.
	resolved := path.Clean(path.Join("/", curDir, p))
	resolved = strings.TrimPrefix(resolved, "/")

	// If it looks like (or is) a markdown doc/folder, route internally.
	if t.shouldRouteToMarkdown(resolved) {
		if !t.repoPathExists(resolved) {
			// Still route, but mark it missing.
			u.Path = "/file/" + resolved
			return []byte(u.String()), false, true
		}
		u.Path = "/file/" + resolved
		return []byte(u.String()), false, false
	}

	// Otherwise treat as repo asset.
	missing = !t.repoPathExists(resolved)
	u.Path = "/repo/" + resolved
	return []byte(u.String()), true, missing
}

func (t *linkRewriter) repoPathExists(rel string) bool {
	// We treat the repo content as untrusted, so keep this conservative and
	// avoid following anything outside root. ResolveRepoPath enforces that.
	abs, cleanRel, err := util.ResolveRepoPath(t.repoRootAbs, rel)
	if err != nil {
		return false
	}
	if t.ignore != nil && cleanRel != "" {
		if t.ignore.IsIgnored(cleanRel, false) || t.ignore.IsIgnored(cleanRel, true) {
			return false
		}
	}
	_, err = util.Stat(abs)
	return err == nil
}

func (t *linkRewriter) shouldRouteToMarkdown(rel string) bool {
	// Fast heuristic first.
	if util.LooksLikeMarkdownPath(rel) {
		return true
	}

	// If the path exists as a directory in the repo, treat it as a doc target
	// (README.md resolution like index.html). This also fixes directory names
	// that contain dots (e.g. docs/v1.0).
	abs := filepath.Join(t.repoRootAbs, filepath.FromSlash(rel))
	if st, err := util.Stat(abs); err == nil && st.IsDir() {
		return true
	}

	// If it exists and is a markdown file, treat it as a doc target.
	if st, err := util.Stat(abs); err == nil && !st.IsDir() {
		return util.IsMarkdownFileName(path.Base(rel))
	}

	return false
}
