package ui

import (
	"strings"
	"testing"
)

func renderInline(s string) string { return NewMarkdownStream(DefaultScheme()).renderInline(s) }
func renderLine(s string) string   { return NewMarkdownStream(DefaultScheme()).renderLine(s) }

func stripANSI(s string) string {
	var b strings.Builder
	state := 0
	for _, r := range s {
		switch state {
		case 0:
			if r == 0x1b {
				state = 1
			} else {
				b.WriteRune(r)
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
	return b.String()
}

func TestRenderInlineBackticksStripped(t *testing.T) {
	got := stripANSI(renderInline("Use `main.go` here."))
	want := "Use main.go here."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineBackticksInsideBold(t *testing.T) {
	got := stripANSI(renderInline("**Inspect `main.go`**: ok"))
	want := "Inspect main.go: ok"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineNestedBoldItalic(t *testing.T) {
	got := stripANSI(renderInline("**outer *inner* end**"))
	want := "outer inner end"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineUnicodeBacktickLookalikes(t *testing.T) {
	cases := []struct{ name, in string }{
		{"left single quote", "Use ‘main.go‘ here."},
		{"reversed prime", "Use ‵main.go‵ here."},
		{"high reversed 9", "Use ‛main.go‛ here."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(renderInline(tc.in))
			want := "Use main.go here."
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestRenderInlineCurlyApostropheAsCodeMarker(t *testing.T) {
	got := stripANSI(renderInline("Inspect ’main.go’ here."))
	want := "Inspect main.go here."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineCurlyApostrophePreservedBetweenLetters(t *testing.T) {
	got := stripANSI(renderInline("the project’s purpose"))
	want := "the project’s purpose"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineStrikethrough(t *testing.T) {
	got := stripANSI(renderInline("~~deleted~~ kept"))
	want := "deleted kept"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineLink(t *testing.T) {
	got := stripANSI(renderInline("see [docs](https://x.com) here"))
	want := "see docs (https://x.com) here"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineItalicUnderscore(t *testing.T) {
	got := stripANSI(renderInline("_emph_ word"))
	want := "emph word"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineBoldUnderscore(t *testing.T) {
	got := stripANSI(renderInline("__strong__ word"))
	want := "strong word"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderInlineUnmatchedBacktickPreserved(t *testing.T) {
	got := stripANSI(renderInline("a ` lone tick"))
	want := "a ` lone tick"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderLineHeading(t *testing.T) {
	got := stripANSI(renderLine("## Hello"))
	want := "Hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderLineBullet(t *testing.T) {
	got := stripANSI(renderLine("- item"))
	want := "• item"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderLineOrdered(t *testing.T) {
	got := stripANSI(renderLine("3. step"))
	want := "step"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderLineBlockquote(t *testing.T) {
	got := stripANSI(renderLine("> quoted"))
	want := "│ quoted"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStreamHorizontalRule(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	got := stripANSI(m.Write("---\n") + m.Flush())
	if !strings.Contains(got, strings.Repeat("─", 40)) {
		t.Errorf("expected horizontal rule, got %q", got)
	}
}

func TestStreamFencedCodeBlock(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	in := "```go\nfunc main() {}\n```\n"
	got := stripANSI(m.Write(in) + m.Flush())
	if !strings.Contains(got, "func main()") {
		t.Errorf("expected code body present, got %q", got)
	}
	if strings.Contains(got, "```") {
		t.Errorf("expected fence markers stripped, got %q", got)
	}
}

func TestStreamSplitDeltas(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	out := ""
	out += m.Write("Use `main")
	out += m.Write(".go` here.\n")
	out += m.Flush()
	got := stripANSI(out)
	want := "Use main.go here.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseTableRow(t *testing.T) {
	cells := parseTableRow("| a | b | c |")
	want := []string{"a", "b", "c"}
	if len(cells) != len(want) {
		t.Fatalf("got %d cells, want %d", len(cells), len(want))
	}
	for i := range cells {
		if cells[i] != want[i] {
			t.Errorf("cell %d: got %q, want %q", i, cells[i], want[i])
		}
	}
}

func TestIsTableSeparator(t *testing.T) {
	yes := []string{
		"|---|---|",
		"| --- | --- |",
		"|:---|---:|",
		"|:---:|:---:|",
	}
	no := []string{
		"| a | b |",
		"|--|--|",
		"|---abc|---|",
		"",
	}
	for _, s := range yes {
		if !isTableSeparator(s) {
			t.Errorf("expected separator: %q", s)
		}
	}
	for _, s := range no {
		if isTableSeparator(s) {
			t.Errorf("expected NOT separator: %q", s)
		}
	}
}

func TestStreamTable(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	in := "| Tool | Description |\n|------|-------------|\n| read | Read file |\n| write | Write file |\n\n"
	got := stripANSI(m.Write(in) + m.Flush())
	for _, want := range []string{"Tool", "Description", "read", "Read file", "write", "Write file", "┌", "├", "└"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "|------|") {
		t.Errorf("separator line should not appear literally, got:\n%s", got)
	}
}

func TestStreamTableCellWithBackticks(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	in := "| Tool | File |\n|------|------|\n| read | `main.go` |\n\n"
	got := stripANSI(m.Write(in) + m.Flush())
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected main.go in cell, got:\n%s", got)
	}
	if strings.Contains(got, "`main.go`") {
		t.Errorf("backticks should be stripped inside table cells, got:\n%s", got)
	}
}

func TestStreamPipeLineWithoutSeparatorTreatedAsPlain(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	in := "run: cat foo | grep bar\nnext line\n"
	got := stripANSI(m.Write(in) + m.Flush())
	if !strings.Contains(got, "cat foo | grep bar") {
		t.Errorf("plain pipe should pass through, got:\n%s", got)
	}
	if !strings.Contains(got, "next line") {
		t.Errorf("following line should render, got:\n%s", got)
	}
}

func TestStreamTableSplitAcrossDeltas(t *testing.T) {
	m := NewMarkdownStream(DefaultScheme())
	out := ""
	out += m.Write("| a | b |\n|---|")
	out += m.Write("---|\n| 1 | 2 |\n\n")
	out += m.Flush()
	got := stripANSI(out)
	for _, want := range []string{"a", "b", "1", "2", "┌", "└"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"short fits", "hello", 10, []string{"hello"}},
		{"wraps at word boundary", "one two three four", 8, []string{"one two", "three", "four"}},
		{"forces break for long word", "supercalifragilistic", 5, []string{"super", "calif", "ragil", "istic"}},
		{"empty", "", 10, []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapText(tc.in, tc.width)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestStreamTableWrapsLongCells(t *testing.T) {
	prev := termWidth
	SetTermWidth(80)
	defer SetTermWidth(prev)

	m := NewMarkdownStream(DefaultScheme())
	long := "github.com/original-flipster69/koko/internal/privacy"
	in := "| Type | Path | Description |\n|------|------|-------------|\n| Internal | " + long + " | Privacy controls (e.g., data redaction, sanitization). |\n\n"
	got := stripANSI(m.Write(in) + m.Flush())
	for _, line := range strings.Split(got, "\n") {
		if line == "" {
			continue
		}
		if visibleWidth(line) > 80 {
			t.Errorf("row exceeds terminal width 80: %d chars: %q", visibleWidth(line), line)
		}
	}
	if !strings.Contains(got, "Privacy controls") {
		t.Errorf("expected wrapped content present, got:\n%s", got)
	}
	if !strings.Contains(got, "sanitization") {
		t.Errorf("expected later part of wrapped content present, got:\n%s", got)
	}
}

func TestWrapTextPreservesStyleAcrossForceBreak(t *testing.T) {
	code := fg(PureOrange)
	in := code + "terminal" + Reset
	got := wrapText(in, 7)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}
	for i, line := range got {
		if !strings.Contains(line, code) {
			t.Errorf("line %d missing color opener: %q", i, line)
		}
		if !strings.HasSuffix(line, Reset) {
			t.Errorf("line %d missing trailing Reset: %q", i, line)
		}
	}
}

func TestStreamTableMarkersNotSplitByWrap(t *testing.T) {
	prev := termWidth
	SetTermWidth(60)
	defer SetTermWidth(prev)

	m := NewMarkdownStream(DefaultScheme())
	in := "| Pkg | Notes |\n|-----|-------|\n| `privacy` | Privacy controls (e.g., data redaction). |\n\n"
	got := stripANSI(m.Write(in) + m.Flush())
	if strings.Contains(got, "`privacy") || strings.Contains(got, "privacy`") {
		t.Errorf("backticks should not appear literally after wrap, got:\n%s", got)
	}
	if !strings.Contains(got, "privacy") {
		t.Errorf("cell content should still appear, got:\n%s", got)
	}
}

func TestStreamTableUsesAvailableWidth(t *testing.T) {
	prev := termWidth
	SetTermWidth(200)
	defer SetTermWidth(prev)

	m := NewMarkdownStream(DefaultScheme())
	in := "| A | B | C |\n|---|---|---|\n| short | also short | medium length cell |\n\n"
	got := stripANSI(m.Write(in) + m.Flush())
	maxLine := 0
	for _, line := range strings.Split(got, "\n") {
		if w := visibleWidth(line); w > maxLine {
			maxLine = w
		}
	}
	if maxLine > 200 {
		t.Errorf("table exceeds 200 cols: %d", maxLine)
	}
}

func TestVisibleWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"hello", 5},
		{"\x1b[31mhello\x1b[0m", 5},
		{"\x1b[1mbold\x1b[0m text", 9},
		{"", 0},
	}
	for _, tc := range cases {
		if got := visibleWidth(tc.in); got != tc.want {
			t.Errorf("visibleWidth(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
