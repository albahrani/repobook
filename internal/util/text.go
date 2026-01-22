package util

import (
	"strings"

	ast "github.com/yuin/goldmark/ast"
)

func ExtractNodeText(n ast.Node) string {
	// NOTE: goldmark text segments require the original source to extract
	// meaningful text. Prefer ExtractNodeTextWithSource.
	return ""
}

func ExtractNodeTextWithSource(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := node.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		}
		if t, ok := node.(*ast.String); ok {
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}
