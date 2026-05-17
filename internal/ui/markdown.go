package ui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

const (
	defaultTermWidth    = 100
	minTableColWidth    = 6
	tableMargin         = 2
	horizontalRuleWidth = 40
	maxHeadingLevel     = 6
	codeStyle           = "dracula"
)

var termWidth = defaultTermWidth

func SetTermWidth(w int) {
	if w > 0 {
		termWidth = w
	}
}

type blockState int

const (
	stateNormal blockState = iota
	stateInFence
	statePendingTable
	stateInTable
)

type MarkdownStream struct {
	buf            strings.Builder
	state          blockState
	fenceLang      string
	fenceBuf       strings.Builder
	pendingTableHd string
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
	var out strings.Builder
	if rem != "" {
		out.WriteString(m.renderBlock(rem))
	}
	out.WriteString(m.flushTable())
	out.WriteString(m.flushPending())
	if m.state == stateInFence && m.fenceBuf.Len() > 0 {
		out.WriteString(highlightCode(m.fenceLang, m.fenceBuf.String()))
		m.fenceBuf.Reset()
	}
	m.state = stateNormal
	return out.String()
}

func (m *MarkdownStream) flushTable() string {
	if m.state != stateInTable {
		return ""
	}
	out := renderTable(m.tableHeaders, m.tableRows)
	m.state = stateNormal
	m.tableHeaders = nil
	m.tableRows = nil
	return out
}

func (m *MarkdownStream) flushPending() string {
	if m.state != statePendingTable {
		return ""
	}
	line := m.pendingTableHd
	m.pendingTableHd = ""
	m.state = stateNormal
	return renderLine(line) + "\n"
}

func (m *MarkdownStream) endBlockState() string {
	switch m.state {
	case stateInTable:
		return m.flushTable()
	case statePendingTable:
		return m.flushPending()
	}
	return ""
}

func (m *MarkdownStream) renderBlock(block string) string {
	var out strings.Builder
	for _, line := range strings.SplitAfter(block, "\n") {
		if line == "" {
			continue
		}
		hasNL := strings.HasSuffix(line, "\n")
		body := strings.TrimRight(line, "\n")
		trimmed := strings.TrimSpace(body)

		if body == "" {
			out.WriteString(m.endBlockState())
			if hasNL {
				if m.state == stateInFence {
					m.fenceBuf.WriteString("\n")
				} else {
					out.WriteString("\n")
				}
			}
			continue
		}

		if isFenceMarker(trimmed) {
			out.WriteString(m.endBlockState())
			if m.state == stateInFence {
				out.WriteString(highlightCode(m.fenceLang, m.fenceBuf.String()))
				m.fenceBuf.Reset()
				m.fenceLang = ""
				m.state = stateNormal
			} else {
				m.fenceLang = fenceLanguage(trimmed)
				m.fenceBuf.Reset()
				m.state = stateInFence
			}
			continue
		}

		if m.state == stateInFence {
			m.fenceBuf.WriteString(body)
			if hasNL {
				m.fenceBuf.WriteString("\n")
			}
			continue
		}

		if isHorizontalRule(trimmed) {
			out.WriteString(m.endBlockState())
			out.WriteString(Dim + Gray + strings.Repeat("─", horizontalRuleWidth) + Reset)
			if hasNL {
				out.WriteString("\n")
			}
			continue
		}

		if m.state == stateInTable {
			if looksLikeTableRow(trimmed) {
				m.tableRows = append(m.tableRows, parseTableRow(trimmed))
				continue
			}
			out.WriteString(m.flushTable())
		}

		if m.state == statePendingTable {
			if isTableSeparator(trimmed) {
				m.tableHeaders = parseTableRow(m.pendingTableHd)
				m.pendingTableHd = ""
				m.state = stateInTable
				continue
			}
			out.WriteString(m.flushPending())
		}

		if looksLikeTableRow(trimmed) && len(parseTableRow(trimmed)) >= 2 {
			m.pendingTableHd = body
			m.state = statePendingTable
			continue
		}

		out.WriteString(renderLine(body))
		if hasNL {
			out.WriteString("\n")
		}
	}
	return out.String()
}

func isFenceMarker(line string) bool {
	if !strings.HasPrefix(line, "```") {
		return false
	}
	for _, r := range line[3:] {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '+', r == '-', r == '_', r == '.':
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
	return strings.Count(s, string(ch)) >= 3
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

	style := styles.Get(codeStyle)
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
		color := Blueberry
		if h >= 2 {
			color = LavenderIndigo
		}
		return indent + Bold + color + renderInline(text) + Reset
	}
	if after, ok := trimListMarker(trimmed); ok {
		return indent + BrightLavender + "•" + Reset + " " + renderInline(after)
	}
	if after, ok := trimOrderedMarker(trimmed); ok {
		return indent + renderInline(after) + Reset
	}
	if strings.HasPrefix(trimmed, "> ") {
		return indent + Gray + "│" + Reset + " " + renderInline(strings.TrimPrefix(trimmed, "> "))
	}
	return indent + renderInline(trimmed)
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' && n < maxHeadingLevel {
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

var backtickLookalikes = strings.NewReplacer(
	"‘", "`",
	"‵", "`",
	"‛", "`",
)

func normalizeCurlyApostrophe(s string) string {
	if !strings.ContainsRune(s, '’') {
		return s
	}
	isAsciiLetter := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	runes := []rune(s)
	for i, r := range runes {
		if r != '’' {
			continue
		}
		prevLetter := i > 0 && isAsciiLetter(runes[i-1])
		nextLetter := i+1 < len(runes) && isAsciiLetter(runes[i+1])
		if !(prevLetter && nextLetter) {
			runes[i] = '`'
		}
	}
	return string(runes)
}

type inlineHandler func(s string, i int) (string, int, bool)

var inlineHandlers []inlineHandler

func init() {
	inlineHandlers = []inlineHandler{
		handleCodeSpan,
		handleDoubleStar,
		handleDoubleUnderscore,
		handleStrikethrough,
		handleSingleStar,
		handleSingleUnderscore,
		handleLink,
	}
}

func renderInline(s string) string {
	s = backtickLookalikes.Replace(s)
	s = normalizeCurlyApostrophe(s)
	var out strings.Builder
	i := 0
	for i < len(s) {
		matched := false
		for _, h := range inlineHandlers {
			if rendered, consumed, ok := h(s, i); ok {
				out.WriteString(rendered)
				i += consumed
				matched = true
				break
			}
		}
		if !matched {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

func handleCodeSpan(s string, i int) (string, int, bool) {
	if s[i] != '`' {
		return "", 0, false
	}
	end := strings.IndexByte(s[i+1:], '`')
	if end < 0 {
		return "", 0, false
	}
	return PureOrange + s[i+1:i+1+end] + Reset, end + 2, true
}

func handleDoubleStar(s string, i int) (string, int, bool) {
	return matchSymmetric(s, i, "**", Bold+Mauve)
}

func handleDoubleUnderscore(s string, i int) (string, int, bool) {
	return matchSymmetric(s, i, "__", Bold+Mauve)
}

func handleStrikethrough(s string, i int) (string, int, bool) {
	return matchSymmetric(s, i, "~~", Strikethrough)
}

func matchSymmetric(s string, i int, marker, style string) (string, int, bool) {
	if !strings.HasPrefix(s[i:], marker) {
		return "", 0, false
	}
	rest := s[i+len(marker):]
	end := strings.Index(rest, marker)
	if end < 0 {
		return "", 0, false
	}
	return style + renderInline(rest[:end]) + Reset, len(marker)*2 + end, true
}

func handleSingleStar(s string, i int) (string, int, bool) {
	return matchSingleEmph(s, i, '*')
}

func handleSingleUnderscore(s string, i int) (string, int, bool) {
	return matchSingleEmph(s, i, '_')
}

func matchSingleEmph(s string, i int, marker byte) (string, int, bool) {
	if s[i] != marker {
		return "", 0, false
	}
	if i+1 >= len(s) || s[i+1] == marker || s[i+1] == ' ' {
		return "", 0, false
	}
	end := strings.IndexByte(s[i+1:], marker)
	if end <= 0 {
		return "", 0, false
	}
	if s[i+end] == ' ' {
		return "", 0, false
	}
	return Italic + renderInline(s[i+1:i+1+end]) + Reset, end + 2, true
}

func handleLink(s string, i int) (string, int, bool) {
	if s[i] != '[' {
		return "", 0, false
	}
	closeBracket := strings.IndexByte(s[i+1:], ']')
	if closeBracket < 0 {
		return "", 0, false
	}
	afterBracket := i + 1 + closeBracket + 1
	if afterBracket >= len(s) || s[afterBracket] != '(' {
		return "", 0, false
	}
	closeParen := strings.IndexByte(s[afterBracket+1:], ')')
	if closeParen < 0 {
		return "", 0, false
	}
	text := s[i+1 : i+1+closeBracket]
	url := s[afterBracket+1 : afterBracket+1+closeParen]
	rendered := Underline + PureOrange + text + Reset + Dim + Gray + " (" + url + ")" + Reset
	return rendered, (afterBracket + 1 + closeParen + 1) - i, true
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

func renderTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	cols := len(headers)
	colW := computeColumnWidths(headers, rows, cols)

	var out strings.Builder
	out.WriteString(tableBorder(colW, "┌", "┬", "┐"))
	out.WriteString(tableRow(headers, colW, true))
	out.WriteString(tableBorder(colW, "├", "┼", "┤"))
	for _, row := range rows {
		out.WriteString(tableRow(row, colW, false))
	}
	out.WriteString(tableBorder(colW, "└", "┴", "┘"))
	return out.String()
}

func computeColumnWidths(headers []string, rows [][]string, cols int) []int {
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
	if sum <= avail {
		return colW
	}
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
	return colW
}

func tableBorder(colW []int, left, mid, right string) string {
	var b strings.Builder
	b.WriteString(Gray + left)
	for i, w := range colW {
		b.WriteString(strings.Repeat("─", w+2))
		if i < len(colW)-1 {
			b.WriteString(mid)
		}
	}
	b.WriteString(right + Reset + "\n")
	return b.String()
}

func tableRow(row []string, colW []int, header bool) string {
	cols := len(colW)
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
			b.WriteString(" " + cell + strings.Repeat(" ", pad) + " " + Gray + "│" + Reset)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func consumeAnsi(rs []rune, i int) (string, int) {
	j := i + 2
	for j < len(rs) && !(rs[j] >= 0x40 && rs[j] <= 0x7e) {
		j++
	}
	if j < len(rs) {
		j++
	}
	return string(rs[i:j]), j
}

func wrapText(s string, width int) []string {
	if width <= 0 || visibleWidth(s) <= width {
		return []string{s}
	}
	return newWrapper(s, width).run()
}

type wrapper struct {
	runes       []rune
	width       int
	hasAnsi     bool
	lines       []string
	cur, word   strings.Builder
	activeStyle strings.Builder
	wordVis     int
	lineVis     int
}

func newWrapper(s string, width int) *wrapper {
	return &wrapper{
		runes:   []rune(s),
		width:   width,
		hasAnsi: strings.Contains(s, "\x1b["),
	}
}

func (w *wrapper) run() []string {
	for i := 0; i < len(w.runes); {
		r := w.runes[i]
		if r == 0x1b && i+1 < len(w.runes) && w.runes[i+1] == '[' {
			seq, next := consumeAnsi(w.runes, i)
			w.word.WriteString(seq)
			i = next
			continue
		}
		if r == ' ' {
			w.appendWord()
			i++
			continue
		}
		w.word.WriteRune(r)
		w.wordVis++
		i++
	}
	w.appendWord()
	w.flushLine()
	if len(w.lines) == 0 {
		return []string{""}
	}
	return w.lines
}

func (w *wrapper) updateStyle(seq string) {
	if seq == "\x1b[0m" || seq == "\x1b[m" {
		w.activeStyle.Reset()
	} else {
		w.activeStyle.WriteString(seq)
	}
}

func (w *wrapper) flushLine() {
	if w.cur.Len() == 0 {
		return
	}
	out := w.cur.String()
	if w.hasAnsi {
		out += Reset
	}
	w.lines = append(w.lines, out)
	w.cur.Reset()
	w.lineVis = 0
	if w.hasAnsi && w.activeStyle.Len() > 0 {
		w.cur.WriteString(w.activeStyle.String())
	}
}

func (w *wrapper) writeWord(wr []rune) {
	for j := 0; j < len(wr); {
		r := wr[j]
		if r == 0x1b && j+1 < len(wr) && wr[j+1] == '[' {
			seq, next := consumeAnsi(wr, j)
			w.cur.WriteString(seq)
			w.updateStyle(seq)
			j = next
			continue
		}
		w.cur.WriteRune(r)
		j++
	}
}

func (w *wrapper) forceBreakWord(wr []rune) {
	seen := 0
	for j := 0; j < len(wr); {
		r := wr[j]
		if r == 0x1b && j+1 < len(wr) && wr[j+1] == '[' {
			seq, next := consumeAnsi(wr, j)
			w.cur.WriteString(seq)
			w.updateStyle(seq)
			j = next
			continue
		}
		if seen == w.width {
			w.flushLine()
			seen = 0
		}
		w.cur.WriteRune(r)
		seen++
		j++
	}
	w.lineVis = seen
}

func (w *wrapper) appendWord() {
	if w.wordVis == 0 {
		return
	}
	wr := []rune(w.word.String())
	sep := 0
	if w.lineVis > 0 {
		sep = 1
	}
	switch {
	case w.lineVis+sep+w.wordVis <= w.width:
		if sep > 0 {
			w.cur.WriteRune(' ')
			w.lineVis++
		}
		w.writeWord(wr)
		w.lineVis += w.wordVis
	case w.lineVis == 0 && w.wordVis > w.width:
		w.forceBreakWord(wr)
	default:
		w.flushLine()
		w.writeWord(wr)
		w.lineVis = w.wordVis
	}
	w.word.Reset()
	w.wordVis = 0
}
