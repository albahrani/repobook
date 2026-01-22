package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcher_IsIgnored(t *testing.T) {
	root := t.TempDir()
	gi := []byte("private/\n*.tmp\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), gi, 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	m, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !m.IsIgnored("private", true) {
		t.Fatalf("expected private dir to be ignored")
	}
	if !m.IsIgnored("private/secret.md", false) {
		t.Fatalf("expected private file to be ignored")
	}
	if !m.IsIgnored("tmp/file.tmp", false) {
		t.Fatalf("expected *.tmp to be ignored")
	}
	if m.IsIgnored("docs/readme.md", false) {
		t.Fatalf("did not expect docs/readme.md to be ignored")
	}
}
