package search

import (
	"os"
	"path/filepath"
	"testing"

	"repobook/internal/ignore"
)

func TestFallback_SearchesMarkdownOnly_RespectsIgnore(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("private/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "private"), 0o755); err != nil {
		t.Fatalf("mkdir private: %v", err)
	}

	// Markdown file should be searched.
	if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte("Alpha appears here\n"), 0o644); err != nil {
		t.Fatalf("write note.md: %v", err)
	}
	// Non-markdown should be ignored.
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("Alpha should not be found\n"), 0o644); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}
	// Ignored markdown should not be searched.
	if err := os.WriteFile(filepath.Join(root, "private", "secret.md"), []byte("Alpha secret\n"), 0o644); err != nil {
		t.Fatalf("write secret.md: %v", err)
	}

	ig, err := ignore.Load(root)
	if err != nil {
		t.Fatalf("ignore.Load: %v", err)
	}

	res, err := Fallback(root, ig, "Alpha", 200)
	if err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}
	if res.Results[0].Path != "docs/note.md" {
		t.Fatalf("expected docs/note.md, got %q", res.Results[0].Path)
	}
}

func TestFallback_SmartCase(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte("alpha\nAlpha\n"), 0o644); err != nil {
		t.Fatalf("write note.md: %v", err)
	}

	// Lowercase query => case-insensitive => matches both lines.
	res, err := Fallback(root, nil, "alpha", 200)
	if err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	if len(res.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res.Results))
	}

	// Uppercase in query => case-sensitive => matches only the 'Alpha' line.
	res2, err := Fallback(root, nil, "Alpha", 200)
	if err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	if len(res2.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res2.Results))
	}
	if res2.Results[0].Line != 2 {
		t.Fatalf("expected match on line 2, got %d", res2.Results[0].Line)
	}
}

func TestFallback_LimitTruncates(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("Alpha\nAlpha\nAlpha\n"), 0o644); err != nil {
		t.Fatalf("write note.md: %v", err)
	}

	res, err := Fallback(root, nil, "Alpha", 2)
	if err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	if len(res.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res.Results))
	}
	if !res.Truncated {
		t.Fatalf("expected truncated")
	}
}
