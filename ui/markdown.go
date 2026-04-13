package ui

import (
	"fmt"
	"strings"
)

type MarkdownStream struct {
	buf     strings.Builder
	inFence bool
}

func NewMarkdownStream() *MarkdownStream {
	return &MarkdownStream{}
}

func (m *MarkdownStream) Write(delta string) string {
	m.buf.WriteString(delta)
	s := m.buf.String()
	idx := strings.LastIndex(s, "\n")
	if idx < 0 {
		return ""
	}
	ready := s[:idx+1]
	m.buf.Reset()
	m.buf.WriteString(s[idx+1:])
	return m.renderBlock(ready)
}

func (m *MarkdownStream) Flush() string {
	rem := m.buf.String()
	m.buf.Reset()
	if rem == "" {
		return ""
	}
	return m.renderBlock(rem)
}

const responseIndent = "  "

func (m *MarkdownStream) renderBlock(block string) string {
	width := diffWidth()
	wrapAt := width - len(responseIndent)
	if wrapAt < 20 {
		wrapAt = 20
	}

	var out strings.Builder
	lines := strings.SplitAfter(block, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		hasNL := strings.HasSuffix(line, "\n")
		body := strings.TrimRight(line, "\n")
		trimmed := strings.TrimSpace(body)

		if body == "" {
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		if strings.HasPrefix(trimmed, "```") {
			m.inFence = !m.inFence
			out.WriteString(responseIndent + Gray + body + Reset)
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		if m.inFence {
			out.WriteString(responseIndent + Cyan + body + Reset)
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		segments := wrapWords(body, wrapAt)
		for i, seg := range segments {
			out.WriteString(responseIndent)
			if i == 0 {
				out.WriteString(renderLine(seg))
			} else {
				out.WriteString(White + renderInline(seg) + Reset)
			}
			if i < len(segments)-1 || hasNL {
				out.WriteString("\n")
			}
		}
	}
	return out.String()
}

func wrapWords(s string, limit int) []string {
	if limit < 1 || runeLen(s) <= limit {
		return []string{s}
	}
	leading := ""
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i > 0 {
		leading = s[:i]
		s = s[i:]
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{leading}
	}
	var lines []string
	var cur strings.Builder
	cur.WriteString(leading)
	curLen := runeLen(leading)
	for _, w := range words {
		wLen := runeLen(w)
		if curLen == runeLen(leading) {
			cur.WriteString(w)
			curLen += wLen
			continue
		}
		if curLen+1+wLen > limit {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
			curLen = wLen
			continue
		}
		cur.WriteString(" ")
		cur.WriteString(w)
		curLen += 1 + wLen
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func renderLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	if h := headingLevel(trimmed); h > 0 {
		text := strings.TrimSpace(trimmed[h:])
		color := BrightPurp
		if h >= 2 {
			color = Purple
		}
		return fmt.Sprintf("%s%s%s%s%s", indent, Bold, color, text, Reset)
	}

	if after, ok := trimListMarker(trimmed); ok {
		return fmt.Sprintf("%s%s•%s %s", indent, Violet, Reset, renderInline(after))
	}

	if strings.HasPrefix(trimmed, "> ") {
		return fmt.Sprintf("%s%s│ %s%s", indent, Gray, strings.TrimPrefix(trimmed, "> "), Reset)
	}

	return indent + renderInline(trimmed)
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' && n < 6 {
		n++
	}
	if n > 0 && n < len(s) && s[n] == ' ' {
		return n
	}
	return 0
}

func trimListMarker(s string) (string, bool) {
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return s[2:], true
	}
	return "", false
}

func renderInline(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				out.WriteString(Cyan)
				out.WriteString(s[i+1 : i+1+end])
				out.WriteString(Reset)
				out.WriteString(White)
				i += end + 2
				continue
			}
		}
		if c == '*' && i+1 < len(s) && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				out.WriteString(Bold)
				out.WriteString(LightPurp)
				out.WriteString(s[i+2 : i+2+end])
				out.WriteString(Reset)
				out.WriteString(White)
				i += end + 4
				continue
			}
		}
		if c == '_' && i+1 < len(s) && s[i+1] == '_' {
			end := strings.Index(s[i+2:], "__")
			if end >= 0 {
				out.WriteString(Bold)
				out.WriteString(LightPurp)
				out.WriteString(s[i+2 : i+2+end])
				out.WriteString(Reset)
				out.WriteString(White)
				i += end + 4
				continue
			}
		}
		out.WriteByte(c)
		i++
	}
	return White + out.String() + Reset
}
