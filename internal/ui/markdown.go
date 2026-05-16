package ui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type MarkdownStream struct {
	buf       strings.Builder
	inFence   bool
	fenceLang string
	fenceBuf  strings.Builder
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
	out := ""
	if rem != "" {
		out = m.renderBlock(rem)
	}
	if m.inFence && m.fenceBuf.Len() > 0 {
		out += highlightCode(m.fenceLang, m.fenceBuf.String())
		m.fenceBuf.Reset()
	}
	m.inFence = false
	return out
}

func isFenceMarker(line string) bool {
	if !strings.HasPrefix(line, "```") {
		return false
	}
	rest := line[3:]
	for _, r := range rest {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '+' || r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func fenceLanguage(line string) string {
	if !strings.HasPrefix(line, "```") {
		return ""
	}
	return strings.TrimSpace(line[3:])
}

func isHorizontalRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	ch := s[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for _, r := range s {
		if r != rune(ch) && r != ' ' {
			return false
		}
	}
	count := strings.Count(s, string(ch))
	return count >= 3
}

func (m *MarkdownStream) renderBlock(block string) string {
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
				if m.inFence {
					m.fenceBuf.WriteString("\n")
				} else {
					out.WriteString("\n")
				}
			}
			continue
		}

		if isFenceMarker(trimmed) {
			if !m.inFence {
				m.inFence = true
				m.fenceLang = fenceLanguage(trimmed)
				m.fenceBuf.Reset()
			} else {
				out.WriteString(highlightCode(m.fenceLang, m.fenceBuf.String()))
				m.fenceBuf.Reset()
				m.inFence = false
				m.fenceLang = ""
			}
			continue
		}

		if m.inFence {
			m.fenceBuf.WriteString(body)
			if hasNL {
				m.fenceBuf.WriteString("\n")
			}
			continue
		}

		if isHorizontalRule(trimmed) {
			rule := Dim + Gray + strings.Repeat("─", 40) + Reset
			out.WriteString(rule)
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		out.WriteString(renderLine(body))
		if hasNL {
			out.WriteString("\n")
		}
	}
	return out.String()
}

func highlightCode(lang, code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return PureOrange + code + Reset
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return PureOrange + code + Reset
	}
	return buf.String()
}

func renderLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]

	if h := headingLevel(trimmed); h > 0 {
		text := strings.TrimSpace(trimmed[h:])
		text = stripBoldMarkers(text)
		color := Blueberry
		if h >= 2 {
			color = LavenderIndigo
		}
		return fmt.Sprintf("%s%s%s%s%s", indent, Bold, color, text, Reset)
	}

	if after, ok := trimListMarker(trimmed); ok {
		return fmt.Sprintf("%s%s•%s %s", indent, BrightLavender, Reset, renderInline(after))
	}

	if after, ok := trimOrderedMarker(trimmed); ok {
		return fmt.Sprintf("%s%s%s", indent, renderInline(after), Reset)
	}

	if strings.HasPrefix(trimmed, "> ") {
		inner := strings.TrimPrefix(trimmed, "> ")
		return fmt.Sprintf("%s%s│%s %s", indent, Gray, Reset, renderInline(inner))
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

func stripBoldMarkers(s string) string {
	if strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") && len(s) > 4 {
		return s[2 : len(s)-2]
	}
	if strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__") && len(s) > 4 {
		return s[2 : len(s)-2]
	}
	return s
}

func trimListMarker(s string) (string, bool) {
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return s[2:], true
	}
	return "", false
}

func trimOrderedMarker(s string) (string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s)-1 && s[i] == '.' && s[i+1] == ' ' {
		return s[i+2:], true
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
				out.WriteString(PureOrange)
				out.WriteString(s[i+1 : i+1+end])
				out.WriteString(Reset)
				i += end + 2
				continue
			}
		}

		if c == '*' && i+1 < len(s) && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				out.WriteString(Bold)
				out.WriteString(Mauve)
				out.WriteString(s[i+2 : i+2+end])
				out.WriteString(Reset)
				i += end + 4
				continue
			}
		}

		if c == '*' && i+1 < len(s) && s[i+1] != '*' && s[i+1] != ' ' {
			end := strings.IndexByte(s[i+1:], '*')
			if end > 0 && s[i+end] != ' ' {
				out.WriteString(Italic)
				out.WriteString(s[i+1 : i+1+end])
				out.WriteString(Reset)
				i += end + 2
				continue
			}
		}

		if c == '_' && i+1 < len(s) && s[i+1] == '_' {
			end := strings.Index(s[i+2:], "__")
			if end >= 0 {
				out.WriteString(Bold)
				out.WriteString(Mauve)
				out.WriteString(s[i+2 : i+2+end])
				out.WriteString(Reset)
				i += end + 4
				continue
			}
		}

		if c == '_' && i+1 < len(s) && s[i+1] != '_' && s[i+1] != ' ' {
			end := strings.IndexByte(s[i+1:], '_')
			if end > 0 && s[i+end] != ' ' {
				out.WriteString(Italic)
				out.WriteString(s[i+1 : i+1+end])
				out.WriteString(Reset)
				i += end + 2
				continue
			}
		}

		if c == '~' && i+1 < len(s) && s[i+1] == '~' {
			end := strings.Index(s[i+2:], "~~")
			if end >= 0 {
				out.WriteString(Strikethrough)
				out.WriteString(s[i+2 : i+2+end])
				out.WriteString(Reset)
				i += end + 4
				continue
			}
		}

		if c == '[' {
			closeBracket := strings.IndexByte(s[i+1:], ']')
			if closeBracket >= 0 {
				afterBracket := i + 1 + closeBracket + 1
				if afterBracket < len(s) && s[afterBracket] == '(' {
					closeParen := strings.IndexByte(s[afterBracket+1:], ')')
					if closeParen >= 0 {
						text := s[i+1 : i+1+closeBracket]
						url := s[afterBracket+1 : afterBracket+1+closeParen]
						out.WriteString(Underline)
						out.WriteString(PureOrange)
						out.WriteString(text)
						out.WriteString(Reset)
						out.WriteString(Dim)
						out.WriteString(Gray)
						out.WriteString(" (")
						out.WriteString(url)
						out.WriteString(")")
						out.WriteString(Reset)
						i = afterBracket + 1 + closeParen + 1
						continue
					}
				}
			}
		}

		out.WriteByte(c)
		i++
	}
	return out.String()
}
