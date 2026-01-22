package ignore

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Matcher wraps .gitignore matching.
//
// Note: This is intentionally best-effort. If there is no .gitignore, it simply
// matches nothing.
type Matcher struct {
	gi *gitignore.GitIgnore
}

func Load(rootAbs string) (*Matcher, error) {
	p := filepath.Join(rootAbs, ".gitignore")
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return &Matcher{}, nil
		}
		return nil, err
	}

	gi, err := gitignore.CompileIgnoreFile(p)
	if err != nil {
		return nil, err
	}
	return &Matcher{gi: gi}, nil
}

func (m *Matcher) IsIgnored(relSlash string, isDir bool) bool {
	if m == nil || m.gi == nil {
		return false
	}

	p := relSlash
	if isDir && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return m.gi.MatchesPath(p)
}
