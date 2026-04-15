package sandbox

import (
	"path/filepath"
	"regexp"
	"strings"
)

type ignoreMatcher struct {
	patterns []string
	regexes  []*regexp.Regexp
}

func newIgnoreMatcher(patterns []string) *ignoreMatcher {
	m := &ignoreMatcher{patterns: patterns}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		re, err := compileIgnorePattern(p)
		if err != nil {
			continue
		}
		m.regexes = append(m.regexes, re)
	}
	return m
}

func (m *ignoreMatcher) matches(relPath string) bool {
	if m == nil || len(m.regexes) == 0 {
		return false
	}
	rel := filepath.ToSlash(relPath)
	base := filepath.Base(rel)
	for _, re := range m.regexes {
		if re.MatchString(rel) || re.MatchString(base) {
			return true
		}
	}
	return false
}

func compileIgnorePattern(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(pattern)
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
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
