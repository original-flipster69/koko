package secrets

import "regexp"

var piiPatterns = []pattern{
	{"email", regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
	{"ssn", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{"phone_us", regexp.MustCompile(`\b(?:\+?1[\s\-.]?)?\(?\d{3}\)?[\s\-.]?\d{3}[\s\-.]?\d{4}\b`)},
	{"credit_card", regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)},
	{"ipv4", regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|[01]?\d\d?)(?:\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)){3}\b`)},
}

func ScanPII(content string) []Match {
	var out []Match
	for _, p := range piiPatterns {
		for _, loc := range p.re.FindAllStringIndex(content, -1) {
			out = append(out, Match{Kind: p.kind, Start: loc[0], End: loc[1]})
		}
	}
	return out
}

func RedactPII(content string) (string, int) {
	matches := ScanPII(content)
	if len(matches) == 0 {
		return content, 0
	}
	sortMatches(matches)
	var b []byte
	b = make([]byte, 0, len(content))
	last := 0
	count := 0
	for _, m := range matches {
		if m.Start < last {
			continue
		}
		b = append(b, content[last:m.Start]...)
		b = append(b, []byte("[REDACTED:"+toUpper(m.Kind)+"]")...)
		last = m.End
		count++
	}
	b = append(b, content[last:]...)
	return string(b), count
}

func RedactAll(content string) (string, int) {
	s, n1 := Redact(content)
	s, n2 := RedactPII(s)
	return s, n1 + n2
}

func toUpper(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		out[i] = c
	}
	return string(out)
}
