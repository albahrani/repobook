package render

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	ast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	gmutil "github.com/yuin/goldmark/util"

	"repobook/internal/util"
)

type Options struct {
	RepoRootAbs string
}

type TOCItem struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

type RenderResult struct {
	Path  string    `json:"path"`
	Title string    `json:"title"`
	HTML  string    `json:"html"`
	TOC   []TOCItem `json:"toc"`
	MTime int64     `json:"mtime"`
}

type Renderer struct {
	rootAbs string
	md      goldmark.Markdown
	policy  *bluemonday.Policy

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	mtime int64
	res   RenderResult
}

func New(opts Options) (*Renderer, error) {
	if opts.RepoRootAbs == "" {
		return nil, fmt.Errorf("RepoRootAbs is required")
	}

	r := &Renderer{
		rootAbs: opts.RepoRootAbs,
		cache:   make(map[string]cached),
	}

	// GitHub-flavored-ish markdown.
	r.md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				gmutil.Prioritized(&linkRewriter{repoRootAbs: r.rootAbs}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // sanitization is applied afterwards
		),
	)

	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("class").OnElements("div", "pre", "code", "span")
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	p.AllowAttrs("rel", "target").OnElements("a")
	// Allow internal links (we still sanitize schemes).
	p.AllowURLSchemes("http", "https", "mailto", "tel")
	r.policy = p

	return r, nil
}

func (r *Renderer) RenderFile(rel string) (RenderResult, error) {
	rel = filepath.ToSlash(rel)
	abs, _, err := util.ResolveRepoPath(r.rootAbs, rel)
	if err != nil {
		return RenderResult{}, err
	}

	st, err := os.Stat(abs)
	if err != nil {
		return RenderResult{}, err
	}
	mtime := st.ModTime().UnixNano()

	r.mu.Lock()
	if c, ok := r.cache[rel]; ok && c.mtime == mtime {
		res := c.res
		r.mu.Unlock()
		return res, nil
	}
	r.mu.Unlock()

	src, err := os.ReadFile(abs)
	if err != nil {
		return RenderResult{}, err
	}

	// Set per-render context for link rewriting.
	ctx := parser.NewContext()
	ctx.Set(linkCtxKeyCurrentRel, rel)

	reader := text.NewReader(src)
	doc := r.md.Parser().Parse(reader, parser.WithContext(ctx))
	toc := extractTOC(doc, src)

	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		return RenderResult{}, err
	}
	htmlOut := r.policy.SanitizeBytes(buf.Bytes())

	title := ""
	for _, it := range toc {
		if it.Level == 1 {
			title = it.Title
			break
		}
	}
	if title == "" {
		title = path.Base(rel)
	}

	res := RenderResult{
		Path:  rel,
		Title: title,
		HTML:  string(htmlOut),
		TOC:   toc,
		MTime: mtime,
	}

	r.mu.Lock()
	r.cache[rel] = cached{mtime: mtime, res: res}
	r.mu.Unlock()

	return res, nil
}

func extractTOC(doc ast.Node, source []byte) []TOCItem {
	items := make([]TOCItem, 0, 32)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		id := ""
		if v, ok := h.AttributeString("id"); ok {
			if s, ok := v.([]byte); ok {
				id = string(s)
			} else if s, ok := v.(string); ok {
				id = s
			}
		}
		title := util.ExtractNodeTextWithSource(h, source)
		if strings.TrimSpace(title) == "" {
			return ast.WalkContinue, nil
		}
		items = append(items, TOCItem{Level: h.Level, ID: id, Title: title})
		return ast.WalkContinue, nil
	})
	return items
}

// small helper to avoid unused import in older Go toolchains
// (kept empty intentionally)
