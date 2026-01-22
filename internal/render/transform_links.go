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
			v.Destination = rewriteURLDest(curDir, v.Destination)
		case *ast.Image:
			v.Destination = rewriteURLDest(curDir, v.Destination)
		}
		return ast.WalkContinue, nil
	})
}

func rewriteURLDest(curDir string, dest []byte) []byte {
	raw := strings.TrimSpace(string(dest))
	if raw == "" {
		return dest
	}
	if strings.HasPrefix(raw, "#") {
		return dest
	}

	u, err := url.Parse(raw)
	if err != nil {
		return dest
	}
	if u.Scheme != "" || u.Host != "" {
		return dest
	}

	p := u.Path
	if p == "" {
		return dest
	}

	// Resolve relative to current file dir.
	resolved := path.Clean(path.Join("/", curDir, p))
	resolved = strings.TrimPrefix(resolved, "/")

	// Directory navigation uses README.md like index.html.
	if strings.HasSuffix(resolved, "/") {
		resolved = strings.TrimSuffix(resolved, "/")
	}

	// If it looks like a markdown doc (or a folder), route internally.
	if util.LooksLikeMarkdownPath(resolved) {
		u.Path = "/file/" + resolved
		return []byte(u.String())
	}

	// Otherwise treat as repo asset.
	u.Path = "/repo/" + resolved
	return []byte(u.String())
}
