package render

import (
    stdhtml "html"
    "strings"

    "github.com/yuin/goldmark/ast"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark/renderer"
    "github.com/yuin/goldmark/text"
    "github.com/yuin/goldmark/util"
)

// DiagramBlock is a synthetic block node rendered as raw HTML.
type DiagramBlock struct {
    ast.BaseBlock
    HTML string
}

var KindDiagramBlock = ast.NewNodeKind("DiagramBlock")

func (n *DiagramBlock) Kind() ast.NodeKind { return KindDiagramBlock }
func (n *DiagramBlock) Dump(source []byte, level int) {
    ast.DumpHelper(n, source, level, map[string]string{"HTML": "(inline)"}, nil)
}
func (n *DiagramBlock) IsRaw() bool { return true }

type diagramTransformer struct{}

func (t *diagramTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
    source := reader.Source()
    // Collect fenced code blocks first.
    var fences []*ast.FencedCodeBlock
    _ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
        if !entering {
            return ast.WalkContinue, nil
        }
        if f, ok := n.(*ast.FencedCodeBlock); ok {
            fences = append(fences, f)
        }
        return ast.WalkContinue, nil
    })

    for _, f := range fences {
        langRaw := string(f.Language(source))
        lang := strings.ToLower(strings.TrimSpace(langRaw))
        if i := strings.IndexByte(lang, ' '); i >= 0 {
            lang = lang[:i]
        }
        content := strings.TrimSpace(string(f.Lines().Value(source)))
        parent := f.Parent()
        if parent == nil {
            continue
        }
        switch lang {
        case "mermaid":
            diag := content
            html := `<div class="mermaid">` + stdhtml.EscapeString(diag) + `</div>`
            parent.ReplaceChild(parent, f, &DiagramBlock{HTML: html})
        }
    }
}

type diagramHTMLRenderer struct{}

func (r *diagramHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
    reg.Register(KindDiagramBlock, r.render)
}

func (r *diagramHTMLRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
    if !entering {
        return ast.WalkContinue, nil
    }
    b, ok := node.(*DiagramBlock)
    if !ok {
        return ast.WalkContinue, nil
    }
    _, _ = w.WriteString(b.HTML)
    return ast.WalkSkipChildren, nil
}
