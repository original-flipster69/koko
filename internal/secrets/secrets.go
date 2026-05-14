package secrets

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Match struct {
	Kind  string
	Start int
	End   int
}

type pattern struct {
	kind     string
	re       *regexp.Regexp
	validate func(string) bool
}

var patterns = []pattern{
	{kind: "aws_access_key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{kind: "aws_secret_key", re: regexp.MustCompile(`(?i)aws_secret[_-]?access[_-]?key["'\s:=]+["']?([A-Za-z0-9/+=]{40})["']?`)},
	{kind: "github_pat", re: regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`)},
	{kind: "github_oauth", re: regexp.MustCompile(`\bgho_[A-Za-z0-9]{36}\b`)},
	{kind: "github_user_to_server", re: regexp.MustCompile(`\bghu_[A-Za-z0-9]{36}\b`)},
	{kind: "github_refresh", re: regexp.MustCompile(`\bghr_[A-Za-z0-9]{36}\b`)},
	{kind: "github_server", re: regexp.MustCompile(`\bghs_[A-Za-z0-9]{36}\b`)},
	{kind: "google_api_key", re: regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`)},
	{kind: "slack_token", re: regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)},
	{kind: "stripe_key", re: regexp.MustCompile(`\b(?:sk|pk|rk)_(?:live|test)_[0-9A-Za-z]{24,}\b`)},
	{kind: "openai_key", re: regexp.MustCompile(`\bsk-[A-Za-z0-9]{32,}\b`)},
	{kind: "anthropic_key", re: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_\-]{32,}\b`)},
	{kind: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`)},
	{kind: "private_key", re: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PR
  KEY-----`)},
	{kind: "generic_secret", re: regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|password|passwd|auth)
  =]+["']?([A-Za-z0-9_\-]{24,})["']?`)},
}

func Scan(content string) []Match {
	var matches []Match
	for _, p := range patterns {
		for _, loc := range p.re.FindAllStringIndex(content, -1) {
			if p.validate != nil && !p.validate(content[loc[0]:loc[1]]) {
				continue
			}
			matches = append(matches, Match{Kind: p.kind, Start: loc[0], End: loc[1]})
		}
	}
	return matches
}

func Redact(content string) (string, int) {
	matches := Scan(content)
	if len(matches) == 0 {
		return content, 0
	}
	var b strings.Builder
	b.Grow(len(content))
	last := 0
	count := 0
	sortMatches(matches)
	for _, m := range matches {
		if m.Start < last {
			continue
		}
		b.WriteString(content[last:m.Start])
		b.WriteString(fmt.Sprintf("[REDACTED:%s]", strings.ToUpper(m.Kind)))
		last = m.End
		count++
	}
	b.WriteString(content[last:])
	return b.String(), count
}

func sortMatches(matches []Match) {
	sort.Slice(matches, func(i, j int) bool { return matches[i].Start < matches[j].Start })
}
