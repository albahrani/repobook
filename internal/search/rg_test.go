package search

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRipgrep_NotFound(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", "")
	_, err := Ripgrep(root, "x", 10)
	if err != ErrRipgrepNotFound {
		t.Fatalf("expected ErrRipgrepNotFound, got %v", err)
	}
}

func TestRipgrep_FindsMatches_WhenAvailable(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("Hello Alpha\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res, err := Ripgrep(root, "Alpha", 50)
	if err != nil {
		t.Fatalf("Ripgrep: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}
	if res.Results[0].Path != "a.md" {
		t.Fatalf("expected path a.md, got %q", res.Results[0].Path)
	}
	if res.Results[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", res.Results[0].Line)
	}
}
