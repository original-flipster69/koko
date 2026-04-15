package ignore

import (
	"os"
	"path/filepath"
	"strings"
)

type Matcher struct {
	patterns []pattern
}

type pattern struct {
	negated bool
	dirOnly bool
	glob    string
}

func LoadGitignore(root string) *Matcher {
	m := &Matcher{}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := pattern{}
		if strings.HasPrefix(line, "!") {
			p.negated = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		p.glob = line
		m.patterns = append(m.patterns, p)
	}
	return m
}

func (m *Matcher) IsIgnored(relPath string, isDir bool) bool {
	if m == nil || len(m.patterns) == 0 {
		return false
	}
	ignored := false
	name := filepath.Base(relPath)
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		matched := false
		if strings.Contains(p.glob, "/") {
			matched, _ = filepath.Match(p.glob, relPath)
		} else {
			matched, _ = filepath.Match(p.glob, name)
		}
		if matched {
			ignored = !p.negated
		}
	}
	return ignored
}
