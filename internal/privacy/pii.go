package privacy

import (
	"net"
	"regexp"
	"strings"
)

var piiPatterns = []pattern{
	{kind: "email", re: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
	{kind: "ssn", re: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{kind: "phone_us", re: regexp.MustCompile(`\b(?:\+?1[\s\-.]?)?\(?[2-9]\d{2}\)?[\s\-.]?[2-9]\d{2}[\s\-.]?\d{4}\b`)},
	{kind: "credit_card", re: regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`), validate: validCreditCard},
	{kind: "ipv4", re: regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|[01]?\d\d?)(?:\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)){3}\b`), validate: isPublicIPv4},
}

func ScanPII(content string) []Match {
	var out []Match
	for _, p := range piiPatterns {
		for _, loc := range p.re.FindAllStringIndex(content, -1) {
			if p.validate != nil && !p.validate(content[loc[0]:loc[1]]) {
				continue
			}
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
	var b strings.Builder
	b.Grow(len(content))
	last := 0
	count := 0
	for _, m := range matches {
		if m.Start < last {
			continue
		}
		b.WriteString(content[last:m.Start])
		b.WriteString("[REDACTED:" + strings.ToUpper(m.Kind) + "]")
		last = m.End
		count++
	}
	b.WriteString(content[last:])
	return b.String(), count
}

func RedactAll(content string) (string, int) {
	s, n1 := Redact(content)
	s, n2 := RedactPII(s)
	return s, n1 + n2
}

func validCreditCard(s string) bool {
	digits := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

func isPublicIPv4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}
