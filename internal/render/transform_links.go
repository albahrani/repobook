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
			dest, openNewTab := t.rewriteURLDest(curDir, v.Destination)
			v.Destination = dest
			if openNewTab {
				// For non-markdown repo assets, open in a new tab.
				v.SetAttributeString("target", []byte("_blank"))
				v.SetAttributeString("rel", []byte("noopener"))
			}
		case *ast.Image:
			dest, _ := t.rewriteURLDest(curDir, v.Destination)
			v.Destination = dest
		}
		return ast.WalkContinue, nil
	})
}

func (t *linkRewriter) rewriteURLDest(curDir string, dest []byte) ([]byte, bool) {
	raw := strings.TrimSpace(string(dest))
	if raw == "" {
		return dest, false
	}
	if strings.HasPrefix(raw, "#") {
		return dest, false
	}

	u, err := url.Parse(raw)
	if err != nil {
		return dest, false
	}
	if u.Scheme != "" || u.Host != "" {
		return dest, false
	}

	p := u.Path
	if p == "" {
		return dest, false
	}

	// Resolve relative to current file dir.
	resolved := path.Clean(path.Join("/", curDir, p))
	resolved = strings.TrimPrefix(resolved, "/")

	// If it looks like (or is) a markdown doc/folder, route internally.
	if t.shouldRouteToMarkdown(resolved) {
		u.Path = "/file/" + resolved
		return []byte(u.String()), false
	}

	// Otherwise treat as repo asset.
	u.Path = "/repo/" + resolved
	return []byte(u.String()), true
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
