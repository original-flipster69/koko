package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

var splashFrame = lipgloss.NewStyle().
	Border(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("99")).
	Padding(0, 1)

var splashTitle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("99"))

var splashTagline = lipgloss.NewStyle().
	Italic(true)

func Splash(mascot, provider, model, sandbox, version string, detected []string) string {
	left := strings.TrimRight(mascot, "\n")

	rightLines := []string{
		"",
		splashTitle.Render(" k o k o "),
		splashTagline.Render("  secure coding assistant"),
		"",
		Info("version ", version),
		Info("provider", provider),
		Info("model   ", model),
		Info("sandbox ", sandbox),
	}
	if len(detected) > 0 {
		rightLines = append(rightLines, Info("stack", strings.Join(detected, ", ")))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Center, left, "    ", strings.Join(rightLines, "\n"))
	return splashFrame.Render(body) + "\n"
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
