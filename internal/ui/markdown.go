package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type MarkdownStream struct {
	buf            strings.Builder
	inFence        bool
	fenceLang      string
	fenceBuf       strings.Builder
	pendingTableHd string
	inTable        bool
	tableHeaders   []string
	tableRows      [][]string
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
	out += m.flushTable()
	out += m.flushPendingTableHd()
	if m.inFence && m.fenceBuf.Len() > 0 {
		out += highlightCode(m.fenceLang, m.fenceBuf.String())
		m.fenceBuf.Reset()
	}
	m.inFence = false
	return out
}

func (m *MarkdownStream) flushTable() string {
	if !m.inTable {
		return ""
	}
	out := renderTable(m.tableHeaders, m.tableRows)
	m.inTable = false
	m.tableHeaders = nil
	m.tableRows = nil
	return out
}

func (m *MarkdownStream) flushPendingTableHd() string {
	if m.pendingTableHd == "" {
		return ""
	}
	line := m.pendingTableHd
	m.pendingTableHd = ""
	return renderLine(line) + "\n"
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
			out.WriteString(m.flushTable())
			out.WriteString(m.flushPendingTableHd())
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
			out.WriteString(m.flushTable())
			out.WriteString(m.flushPendingTableHd())
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
			out.WriteString(m.flushTable())
			out.WriteString(m.flushPendingTableHd())
			rule := Dim + Gray + strings.Repeat("─", 40) + Reset
			out.WriteString(rule)
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		if m.inTable {
			if looksLikeTableRow(trimmed) {
				m.tableRows = append(m.tableRows, parseTableRow(trimmed))
				continue
			}
			out.WriteString(m.flushTable())
		}

		if m.pendingTableHd != "" {
			if isTableSeparator(trimmed) {
				m.inTable = true
				m.tableHeaders = parseTableRow(m.pendingTableHd)
				m.pendingTableHd = ""
				continue
			}
			out.WriteString(renderLine(m.pendingTableHd))
			out.WriteString("\n")
			m.pendingTableHd = ""
		}

		if looksLikeTableRow(trimmed) && len(parseTableRow(trimmed)) >= 2 {
			m.pendingTableHd = body
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

func looksLikeTableRow(line string) bool {
	return strings.Contains(line, "|")
}

func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isTableSeparator(line string) bool {
	cells := parseTableRow(line)
	if len(cells) < 1 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimPrefix(c, ":")
		c = strings.TrimSuffix(c, ":")
		if len(c) < 3 || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

func visibleWidth(s string) int {
	w := 0
	state := 0
	for _, r := range s {
		switch state {
		case 0:
			if r == 0x1b {
				state = 1
			} else {
				w++
			}
		case 1:
			if r == '[' {
				state = 2
			} else {
				state = 0
			}
		case 2:
			if r >= 0x40 && r <= 0x7e {
				state = 0
			}
		}
	}
	return w
}

const (
	defaultTermWidth = 100
	minTableColWidth = 6
	tableMargin      = 2
)

var termWidth = defaultTermWidth

func SetTermWidth(w int) {
	if w > 0 {
		termWidth = w
	}
}

func wrapText(s string, width int) []string {
	if width <= 0 || visibleWidth(s) <= width {
		return []string{s}
	}
	hasAnsi := strings.Contains(s, "\x1b[")

	var lines []string
	var cur, word strings.Builder
	var activeStyle strings.Builder
	var wordVis, lineVis int

	consumeAnsi := func(rs []rune, i int) (string, int) {
		j := i + 2
		for j < len(rs) && !(rs[j] >= 0x40 && rs[j] <= 0x7e) {
			j++
		}
		if j < len(rs) {
			j++
		}
		return string(rs[i:j]), j
	}

	updateStyle := func(seq string) {
		if seq == "\x1b[0m" || seq == "\x1b[m" {
			activeStyle.Reset()
		} else {
			activeStyle.WriteString(seq)
		}
	}

	flushLine := func() {
		if cur.Len() == 0 {
			return
		}
		out := cur.String()
		if hasAnsi {
			out += Reset
		}
		lines = append(lines, out)
		cur.Reset()
		lineVis = 0
		if hasAnsi && activeStyle.Len() > 0 {
			cur.WriteString(activeStyle.String())
		}
	}

	writeWord := func(wr []rune) {
		j := 0
		for j < len(wr) {
			r := wr[j]
			if r == 0x1b && j+1 < len(wr) && wr[j+1] == '[' {
				seq, next := consumeAnsi(wr, j)
				cur.WriteString(seq)
				updateStyle(seq)
				j = next
				continue
			}
			cur.WriteRune(r)
			j++
		}
	}

	appendWord := func() {
		if wordVis == 0 {
			return
		}
		wr := []rune(word.String())
		sep := 0
		if lineVis > 0 {
			sep = 1
		}
		if lineVis+sep+wordVis <= width {
			if sep > 0 {
				cur.WriteRune(' ')
				lineVis++
			}
			writeWord(wr)
			lineVis += wordVis
		} else if lineVis == 0 && wordVis > width {
			j, seen := 0, 0
			for j < len(wr) {
				r := wr[j]
				if r == 0x1b && j+1 < len(wr) && wr[j+1] == '[' {
					seq, next := consumeAnsi(wr, j)
					cur.WriteString(seq)
					updateStyle(seq)
					j = next
					continue
				}
				if seen == width {
					flushLine()
					seen = 0
				}
				cur.WriteRune(r)
				seen++
				j++
			}
			lineVis = seen
		} else {
			flushLine()
			writeWord(wr)
			lineVis = wordVis
		}
		word.Reset()
		wordVis = 0
	}

	runes := []rune(s)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			seq, next := consumeAnsi(runes, i)
			word.WriteString(seq)
			i = next
			continue
		}
		if r == ' ' {
			appendWord()
			i++
			continue
		}
		word.WriteRune(r)
		wordVis++
		i++
	}
	appendWord()
	flushLine()

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func renderTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	cols := len(headers)

	colW := make([]int, cols)
	for i, h := range headers {
		if w := visibleWidth(h); w > colW[i] {
			colW[i] = w
		}
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			if w := visibleWidth(row[i]); w > colW[i] {
				colW[i] = w
			}
		}
	}

	avail := termWidth - (3*cols + 1) - tableMargin
	if avail < cols*minTableColWidth {
		avail = cols * minTableColWidth
	}
	sum := 0
	for _, w := range colW {
		sum += w
	}
	if sum > avail {
		used := 0
		for i := range colW {
			colW[i] = avail * colW[i] / sum
			if colW[i] < minTableColWidth {
				colW[i] = minTableColWidth
			}
			used += colW[i]
		}
		for i := range colW {
			if used >= avail {
				break
			}
			colW[i]++
			used++
		}
	}

	border := func(left, mid, right string) string {
		var b strings.Builder
		b.WriteString(Gray + left)
		for i, w := range colW {
			b.WriteString(strings.Repeat("─", w+2))
			if i < cols-1 {
				b.WriteString(mid)
			}
		}
		b.WriteString(right + Reset + "\n")
		return b.String()
	}

	dataRow := func(row []string, header bool) string {
		wrapped := make([][]string, cols)
		for i := 0; i < cols; i++ {
			raw := ""
			if i < len(row) {
				raw = row[i]
			}
			rendered := renderInline(raw)
			if header {
				rendered = Bold + Mauve + rendered + Reset
			}
			wrapped[i] = wrapText(rendered, colW[i])
		}
		maxLines := 1
		for _, c := range wrapped {
			if len(c) > maxLines {
				maxLines = len(c)
			}
		}
		var b strings.Builder
		for line := 0; line < maxLines; line++ {
			b.WriteString(Gray + "│" + Reset)
			for i := 0; i < cols; i++ {
				cell := ""
				if line < len(wrapped[i]) {
					cell = wrapped[i][line]
				}
				pad := colW[i] - visibleWidth(cell)
				if pad < 0 {
					pad = 0
				}
				b.WriteString(" " + cell)
				if pad > 0 {
					b.WriteString(strings.Repeat(" ", pad))
				}
				b.WriteString(" " + Gray + "│" + Reset)
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	var out strings.Builder
	out.WriteString(border("┌", "┬", "┐"))
	out.WriteString(dataRow(headers, true))
	out.WriteString(border("├", "┼", "┤"))
	for _, row := range rows {
		out.WriteString(dataRow(row, false))
	}
	out.WriteString(border("└", "┴", "┘"))
	return out.String()
}

var backtickLookalikes = strings.NewReplacer(
	"‘", "`",
	"‵", "`",
	"‛", "`",
)

func normalizeCurlyApostrophe(s string) string {
	if !strings.ContainsRune(s, '’') {
		return s
	}
	runes := []rune(s)
	for i, r := range runes {
		if r != '’' {
			continue
		}
		prevLetter := i > 0 && unicode.IsLetter(runes[i-1])
		nextLetter := i+1 < len(runes) && unicode.IsLetter(runes[i+1])
		if !(prevLetter && nextLetter) {
			runes[i] = '`'
		}
	}
	return string(runes)
}

func renderInline(s string) string {
	s = backtickLookalikes.Replace(s)
	s = normalizeCurlyApostrophe(s)
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
				out.WriteString(renderInline(s[i+2 : i+2+end]))
				out.WriteString(Reset)
				i += end + 4
				continue
			}
		}

		if c == '*' && i+1 < len(s) && s[i+1] != '*' && s[i+1] != ' ' {
			end := strings.IndexByte(s[i+1:], '*')
			if end > 0 && s[i+end] != ' ' {
				out.WriteString(Italic)
				out.WriteString(renderInline(s[i+1 : i+1+end]))
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
				out.WriteString(renderInline(s[i+2 : i+2+end]))
				out.WriteString(Reset)
				i += end + 4
				continue
			}
		}

		if c == '_' && i+1 < len(s) && s[i+1] != '_' && s[i+1] != ' ' {
			end := strings.IndexByte(s[i+1:], '_')
			if end > 0 && s[i+end] != ' ' {
				out.WriteString(Italic)
				out.WriteString(renderInline(s[i+1 : i+1+end]))
				out.WriteString(Reset)
				i += end + 2
				continue
			}
		}

		if c == '~' && i+1 < len(s) && s[i+1] == '~' {
			end := strings.Index(s[i+2:], "~~")
			if end >= 0 {
				out.WriteString(Strikethrough)
				out.WriteString(renderInline(s[i+2 : i+2+end]))
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
