package policy

import (
	"fmt"
	"regexp"
	"strings"
)

type CmdPolicy struct {
	Allowlist     []string
	DenyPatterns  []string
	denyCompiled  []*regexp.Regexp
	allowedParsed map[string]bool
	requireAllow  bool
}

var DefaultDenyPatterns = []string{
	`\bsudo\b`,
	`\bsu\s`,
	`\bssh\b`,
	`\bscp\b`,
	`\bsftp\b`,
	`\bnc\b`,
	`\bncat\b`,
	`\btelnet\b`,
	`\beval\b`,
	`\bexec\b`,
	`\bsource\b`,
	`\.\s*\/?[^ ]*\.sh\b`,
	"`[^`]+`",
	`\$\([^)]+\)`,
	`\brm\s+-rf?\s+/`,
	`\bchmod\s+\+s\b`,
	`>\s*/dev/(tcp|udp)/`,
	`curl[^|]*\|\s*(?:ba)?sh\b`,
	`wget[^|]*\|\s*(?:ba)?sh\b`,
	`curl[^|]*\|\s*python\b`,
	`wget[^|]*\|\s*python\b`,
	`\bmkfifo\b`,
	`\bdd\s+if=/dev/`,
}

func NewCommandPolicy(allowlist, deny []string) (*CmdPolicy, error) {
	if len(deny) == 0 {
		deny = DefaultDenyPatterns
	}
	compiled := make([]*regexp.Regexp, 0, len(deny))
	for _, p := range deny {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compiling deny pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	allowed := make(map[string]bool, len(allowlist))
	for _, a := range allowlist {
		allowed[a] = true
	}
	return &CmdPolicy{
		Allowlist:     allowlist,
		DenyPatterns:  deny,
		denyCompiled:  compiled,
		allowedParsed: allowed,
		requireAllow:  len(allowlist) > 0,
	}, nil
}

func (p *CmdPolicy) Check(cmd string) error {
	for i, re := range p.denyCompiled {
		if re.MatchString(cmd) {
			return fmt.Errorf("command blocked by deny pattern %q", p.DenyPatterns[i])
		}
	}
	if !p.requireAllow {
		return nil
	}
	first := firstToken(cmd)
	if first == "" {
		return fmt.Errorf("empty command")
	}
	if !p.allowedParsed[first] {
		return fmt.Errorf("command %q not in allowlist (allowed: %s)", first, strings.Join(p.Allowlist, ", "))
	}
	return nil
}

func firstToken(cmd string) string {
	trim := strings.TrimSpace(cmd)
	if trim == "" {
		return ""
	}
	for i := 0; i < len(trim); i++ {
		c := trim[i]
		if c == ' ' || c == '\t' {
			return trim[:i]
		}
	}
	return trim
}
