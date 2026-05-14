package ignore

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Matcher struct {
	patterns []pattern
}

type pattern struct {
	negated bool
	dirOnly bool
	glob    string
	regex   *regexp.Regexp
}

func LoadGitignore(root string) *Matcher {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return &Matcher{}
	}
	return NewFromPatterns(strings.Split(string(data), "\n"))
}

func NewFromPatterns(lines []string) *Matcher {
	m := &Matcher{}
	for _, line := range lines {
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
		if strings.Contains(line, "**") {
			if re, err := compileGlob(line); err == nil {
				p.regex = re
			}
		} else if _, err := filepath.Match(line, ""); err != nil {
			slog.Warn("ignore: skipping malformed pattern", "pattern", line, "err", err)
			continue
		}
		m.patterns = append(m.patterns, p)
	}
	return m
}

func compileGlob(g string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(g) {
		c := g[i]
		switch c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				if i+2 < len(g) && g[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 3
					continue
				}
				b.WriteString(".*")
				i += 2
				continue
			}
			b.WriteString("[^/]*")
			i++
		case '?':
			b.WriteString("[^/]")
			i++
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteRune('\\')
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func (m *Matcher) IsIgnored(relPath string, isDir bool) bool {
	if m == nil || len(m.patterns) == 0 {
		return false
	}
	rel := filepath.ToSlash(relPath)
	name := filepath.Base(rel)
	ignored := false
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		matched := false
		switch {
		case p.regex != nil:
			matched = p.regex.MatchString(rel) || p.regex.MatchString(name)
		case strings.Contains(p.glob, "/"):
			matched, _ = filepath.Match(p.glob, rel)
		default:
			matched, _ = filepath.Match(p.glob, name)
		}
		if matched {
			ignored = !p.negated
		}
	}
	return ignored
}
