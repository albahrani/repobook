package scan

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"repobook/internal/ignore"
)

func TestBuildTree_ReadmeFirst_AndGitignoreRespected(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte("# x\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("private/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	mustWrite("README.md")
	mustWrite("notes.md")
	mustWrite("docs/README.md")
	mustWrite("docs/a.md")
	mustWrite("private/secret.md")

	ig, err := ignore.Load(root)
	if err != nil {
		t.Fatalf("ignore.Load: %v", err)
	}

	tree, err := BuildTree(Options{RootAbs: root, Ignore: ig})
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}

	// Root children should contain docs dir and markdown files; README should be first among files.
	if len(tree.Children) == 0 {
		t.Fatalf("expected children")
	}

	// Ensure ignored dir never appears.
	if findNode(&tree, "private") != nil {
		t.Fatalf("expected private dir to be absent")
	}

	// Ensure README.md is before notes.md at root.
	idxReadme := indexOfChild(tree.Children, "README.md")
	idxNotes := indexOfChild(tree.Children, "notes.md")
	if idxReadme < 0 || idxNotes < 0 {
		t.Fatalf("expected README.md and notes.md in root")
	}
	if idxReadme > idxNotes {
		t.Fatalf("expected README.md before notes.md")
	}

	// Ensure docs directory contains README first.
	docs := findNode(&tree, "docs")
	if docs == nil || docs.Type != "dir" {
		t.Fatalf("expected docs dir")
	}
	idxDocsReadme := indexOfChild(docs.Children, "README.md")
	idxDocsA := indexOfChild(docs.Children, "a.md")
	if idxDocsReadme < 0 || idxDocsA < 0 || idxDocsReadme > idxDocsA {
		t.Fatalf("expected docs/README.md before docs/a.md")
	}
}

func indexOfChild(children []Node, name string) int {
	for i, c := range children {
		if c.Name == name {
			return i
		}
	}
	return -1
}

func findNode(n *Node, name string) *Node {
	if n == nil {
		return nil
	}
	if path.Base(n.Path) == name || n.Name == name {
		return n
	}
	for i := range n.Children {
		if got := findNode(&n.Children[i], name); got != nil {
			return got
		}
	}
	return nil
}
