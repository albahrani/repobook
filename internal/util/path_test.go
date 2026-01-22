package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoPath_PreventsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, _, err := ResolveRepoPath(root, "../x"); err == nil {
		t.Fatalf("expected traversal to fail")
	}
}

func TestResolveMarkdownRel_DirReadme(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res, err := ResolveMarkdownRel(root, "docs")
	if err != nil {
		t.Fatalf("ResolveMarkdownRel: %v", err)
	}
	if res.Rel != "docs/README.md" {
		t.Fatalf("expected docs/README.md, got %q", res.Rel)
	}
}

func TestResolveDefaultReadmeRel_CaseInsensitive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ReadMe.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ResolveDefaultReadmeRel(root)
	if err != nil {
		t.Fatalf("ResolveDefaultReadmeRel: %v", err)
	}
	if got != "ReadMe.md" {
		t.Fatalf("expected ReadMe.md, got %q", got)
	}
}

func TestLooksLikeMarkdownPath_EdgeCases(t *testing.T) {
	if !LooksLikeMarkdownPath("docs") {
		t.Fatalf("expected docs to be treated as markdown target")
	}
	if LooksLikeMarkdownPath("docs/v1.0") {
		// This is intentionally false; directory detection is handled by link rewrite via filesystem check.
		t.Fatalf("expected docs/v1.0 to not be treated as markdown by heuristic")
	}
}
