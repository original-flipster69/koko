package ui

import (
	"fmt"
	"strings"
)

const (
	Reset         = "\033[0m"
	Bold          = "\033[1m"
	Dim           = "\033[2m"
	Italic        = "\033[3m"
	Underline     = "\033[4m"
	Strikethrough = "\033[9m"

	LavenderIndigo = "\033[38;5;135m"
	Mauve          = "\033[38;5;183m"
	Blueberry      = "\033[38;5;99m"
	MediumPurple   = "\033[38;5;98m"
	BrightLavender = "\033[38;5;141m"
	DarkViolet     = "\033[38;5;55m"

	PureViolet     = "\033[38;5;93m"
	ElectricPurple = "\033[38;5;129m"

	Gray  = "\033[38;5;243m"
	White = "\033[38;5;255m"

	Red        = "\033[38;5;197m"
	Green      = "\033[38;5;114m"
	PureOrange = "\033[38;5;214m"
)

func visibleWidth(s string) int {
	w := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' || r == 'K' || r == 'H' || r == 'J' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		w++
	}
	return w
}

func Splash(provider, model, sandbox, version string, detected []string) string {
	left := strings.Split(strings.TrimRight(Mascot(), "\n"), "\n")

	title := fmt.Sprintf("%s%s k o k o %s", Bold, Blueberry, Reset)
	tagline := fmt.Sprintf("%s  secure coding assistant%s", Italic, Reset)

	var right []string
	right = append(right, "")
	right = append(right, title)
	right = append(right, tagline)
	right = append(right, "")
	right = append(right, Info("version ", version))
	right = append(right, Info("provider", provider))
	right = append(right, Info("model   ", model))
	right = append(right, Info("sandbox ", sandbox))
	if len(detected) > 0 {
		right = append(right, Info("stack", strings.Join(detected, ", ")))
	}

	leftW := 0
	for _, line := range left {
		if w := visibleWidth(line); w > leftW {
			leftW = w
		}
	}
	rightW := 0
	for _, line := range right {
		if w := visibleWidth(line); w > rightW {
			rightW = w
		}
	}

	rows := len(left)
	if len(right) > rows {
		rows = len(right)
	}
	leftOffset := (rows - len(left)) / 2
	rightOffset := (rows - len(right)) / 2

	gap := "    "
	contentW := leftW + len(gap) + rightW
	innerPadL := " "
	innerPadR := " "
	totalW := len(innerPadL) + contentW + len(innerPadR)

	var out strings.Builder
	out.WriteString(Bold + Blueberry + "╔" + strings.Repeat("═", totalW) + "╗" + Reset + "\n")
	for i := 0; i < rows; i++ {
		var l, r string
		li := i - leftOffset
		if li >= 0 && li < len(left) {
			l = left[li]
		}
		lPad := strings.Repeat(" ", leftW-visibleWidth(l))
		ri := i - rightOffset
		if ri >= 0 && ri < len(right) {
			r = right[ri]
		}
		rPad := strings.Repeat(" ", rightW-visibleWidth(r))
		out.WriteString(Bold + Blueberry + "║" + Reset + innerPadL + l + lPad + gap + r + rPad + innerPadR + Bold + Blueberry + "║" + Reset + "\n")
	}
	out.WriteString(Bold + Blueberry + "╚" + strings.Repeat("═", totalW) + "╝" + Reset + "\n")
	return out.String()
}

func Info(label string, value string) string {
	return fmt.Sprintf("  %s%-9s%s %s%s%s", Bold+BrightLavender, label, Reset, White, value, Reset)
}

func Error(text string) string {
	return fmt.Sprintf("%s%serror:%s %s", Bold, Red, Reset, text)
}

func TokenStats(input, output int) string {
	return fmt.Sprintf("  %s%stokens: %d in / %d out%s", Dim, Gray, input, output, Reset)
}

const (
	diffBgRed   = "\033[48;5;52m"
	diffFgRed   = "\033[38;5;210m"
	diffBgGreen = "\033[48;5;22m"
	diffFgGreen = "\033[38;5;156m"
	diffGutter  = "\033[38;5;240m"
)

func ColorDiff(diffText string) string {
	if diffText == "" {
		return ""
	}
	var out strings.Builder
	var oldLine, newLine int
	var path string
	headerPrinted := false

	for _, line := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(line, "--- a/"):
			path = strings.TrimPrefix(line, "--- a/")
			continue
		case strings.HasPrefix(line, "+++ b/"):
			if p := strings.TrimPrefix(line, "+++ b/"); p != "" {
				path = p
			}
			if !headerPrinted {
				out.WriteString(fmt.Sprintf("  %s%s╭─ %s%s\n", Bold, Blueberry, path, Reset))
				headerPrinted = true
			}
			continue
		case strings.HasPrefix(line, "@@"):
			var oc, nc int
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldLine, &oc, &newLine, &nc)
			out.WriteString(fmt.Sprintf("  %s│ %s%s%s\n", Blueberry, Dim+Gray, line, Reset))
			continue
		case line == "":
			continue
		}

		sign := line[:1]
		content := ""
		if len(line) > 1 {
			content = line[1:]
		}
		content = strings.ReplaceAll(content, "\t", "  ")

		var gutter, bg, fg, prefix string
		switch sign {
		case "-":
			gutter = fmt.Sprintf("%4d     ", oldLine)
			bg, fg, prefix = diffBgRed, diffFgRed, " - "
			oldLine++
		case "+":
			gutter = fmt.Sprintf("     %4d", newLine)
			bg, fg, prefix = diffBgGreen, diffFgGreen, " + "
			newLine++
		default:
			gutter = fmt.Sprintf("%4d %4d", oldLine, newLine)
			bg, fg, prefix = "", Gray, "   "
			oldLine++
			newLine++
		}

		if bg != "" {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s%s\n",
				Blueberry, diffGutter, gutter, bg, fg, prefix, content, Reset))
		} else {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s\n",
				Blueberry, diffGutter, gutter, fg, prefix, content, Reset))
		}
	}

	if headerPrinted {
		out.WriteString(fmt.Sprintf("  %s╰─%s\n", Blueberry, Reset))
	}
	return out.String()
}
