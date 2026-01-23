package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenderer_RenderFile_TOC_Links_Sanitization(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel string, body string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("a.md", "# Title\n\nSee [Docs](docs) and [Note](docs/note.md#h).\n\nDownload [PDF](files/report.pdf).\n\nVisit [Example](https://example.com/x).\n\nEmail [Mail](mailto:test@example.com) and call [Call](tel:+15551212).\n\n<script>alert(1)</script>\n\n![Logo](img/logo.svg)\n")
	mustWrite("docs/README.md", "# Docs\n\nGo to [Note](note.md).\n")
	mustWrite("docs/note.md", "# Note\n\n## H\n\n```go\npackage main\n\nfunc main() {}\n```\n")
	mustWrite("img/logo.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 10 10\"><text x=\"0\" y=\"10\">x</text></svg>")
	mustWrite("files/report.pdf", "%PDF-1.4\n%test\n")

	r, err := New(Options{RepoRootAbs: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := r.RenderFile("a.md")
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	if res.Title != "Title" {
		t.Fatalf("expected title 'Title', got %q", res.Title)
	}
	if len(res.TOC) == 0 || res.TOC[0].Title != "Title" {
		t.Fatalf("expected TOC to include Title")
	}

	if strings.Contains(res.HTML, "<script") {
		t.Fatalf("expected script tags to be sanitized")
	}
	if !strings.Contains(res.HTML, "href=\"/file/docs\"") {
		t.Fatalf("expected directory markdown link to route to /file/docs; html=%q", res.HTML)
	}
	if !strings.Contains(res.HTML, "href=\"/file/docs/note.md#h\"") {
		t.Fatalf("expected markdown link to route to /file/docs/note.md#h")
	}
	if !strings.Contains(res.HTML, "src=\"/repo/img/logo.svg\"") {
		t.Fatalf("expected image to route to /repo/img/logo.svg")
	}
	if !strings.Contains(res.HTML, "href=\"/repo/files/report.pdf\"") {
		t.Fatalf("expected non-markdown link to route to /repo/files/report.pdf")
	}
	if ok, err := regexp.MatchString(`<a[^>]*href="/repo/files/report\.pdf"[^>]*target="_blank"`, res.HTML); err != nil || !ok {
		t.Fatalf("expected non-markdown link to open in new tab")
	}
	if ok, err := regexp.MatchString(`<a[^>]*href="https://example\.com/x"[^>]*target="_blank"`, res.HTML); err != nil || !ok {
		t.Fatalf("expected external HTTP(S) link to open in new tab")
	}
	if ok, err := regexp.MatchString(`<a[^>]*href="mailto:test@example\.com"[^>]*target="_blank"`, res.HTML); err != nil || !ok {
		t.Fatalf("expected mailto link to open in new tab")
	}
	if ok, err := regexp.MatchString(`<a[^>]*href="tel:\+15551212"[^>]*target="_blank"`, res.HTML); err != nil || !ok {
		t.Fatalf("expected tel link to open in new tab")
	}
}

func TestRenderer_RenderFile_SyntaxHighlighting(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.md"), []byte("# Code\n\n```go\npackage main\n\nfunc main() {}\n```\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := New(Options{RepoRootAbs: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := r.RenderFile("code.md")
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}
	if !strings.Contains(res.HTML, "chroma") {
		t.Fatalf("expected chroma classes in HTML")
	}
}
