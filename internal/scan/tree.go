package scan

import (
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"repobook/internal/util"
)

type Options struct {
	RootAbs string
}

type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // repo-relative, forward slashes
	Type     string `json:"type"` // "dir" or "file"
	Children []Node `json:"children,omitempty"`
}

func BuildTree(opts Options) (Node, error) {
	rootAbs, err := filepath.Abs(opts.RootAbs)
	if err != nil {
		return Node{}, err
	}

	// We build a directory tree containing only markdown files and directories
	// that contain markdown (directly or indirectly).
	filesByDir := map[string][]string{} // dirRel -> []fileRel
	dirSet := map[string]struct{}{}     // dirRel

	ignoreDirs := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"vendor":       {},
		".idea":        {},
		".vscode":      {},
	}

	err = filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			name := d.Name()
			if _, ok := ignoreDirs[name]; ok {
				return fs.SkipDir
			}
			return nil
		}

		if !util.IsMarkdownFileName(d.Name()) {
			return nil
		}

		relOS, err := filepath.Rel(rootAbs, p)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(relOS)
		dirRel := path.Dir(rel)
		if dirRel == "." {
			dirRel = ""
		}

		filesByDir[dirRel] = append(filesByDir[dirRel], rel)
		// Mark this directory and all parents as present.
		cur := dirRel
		for {
			dirSet[cur] = struct{}{}
			if cur == "" {
				break
			}
			cur = path.Dir(cur)
			if cur == "." {
				cur = ""
			}
		}
		return nil
	})
	if err != nil {
		return Node{}, err
	}

	root := Node{Name: path.Base(filepath.ToSlash(rootAbs)), Path: "", Type: "dir"}
	root.Children = buildDir("", filesByDir, dirSet)
	return root, nil
}

func buildDir(dirRel string, filesByDir map[string][]string, dirSet map[string]struct{}) []Node {
	// Add subdirectories (only those in dirSet).
	subdirs := make([]string, 0, 32)
	prefix := dirRel
	if prefix != "" {
		prefix += "/"
	}
	for d := range dirSet {
		if d == dirRel {
			continue
		}
		if !strings.HasPrefix(d, prefix) {
			continue
		}
		rest := strings.TrimPrefix(d, prefix)
		if rest == "" || strings.Contains(rest, "/") {
			continue
		}
		subdirs = append(subdirs, d)
	}
	sort.Strings(subdirs)

	nodes := make([]Node, 0, 64)
	for _, sd := range subdirs {
		name := path.Base(sd)
		n := Node{Name: name, Path: sd, Type: "dir"}
		n.Children = buildDir(sd, filesByDir, dirSet)
		nodes = append(nodes, n)
	}

	files := append([]string(nil), filesByDir[dirRel]...)
	sort.Strings(files)
	for _, f := range files {
		nodes = append(nodes, Node{Name: path.Base(f), Path: f, Type: "file"})
	}

	// Prefer README.md first inside a directory.
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.Type != b.Type {
			return a.Type == "dir"
		}
		if a.Type == "file" {
			ar := strings.EqualFold(a.Name, "README.md")
			br := strings.EqualFold(b.Name, "README.md")
			if ar != br {
				return ar
			}
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return nodes
}
