package ui

import (
	"fmt"
	"strconv"
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
)

func fg(code int) string { return fmt.Sprintf("\033[38;5;%dm", code) }
func bg(code int) string { return fmt.Sprintf("\033[48;5;%dm", code) }

var (
	LavenderIndigo = fg(135)
	Mauve          = fg(183)
	Blueberry      = fg(99)
	MediumPurple   = fg(98)
	BrightLavender = fg(141)
	DarkViolet     = fg(55)

	PureViolet     = fg(93)
	ElectricPurple = fg(129)

	Gray  = fg(243)
	White = fg(255)

	Red        = fg(197)
	Green      = fg(114)
	PureOrange = fg(214)
)

var (
	diffBgRed   = bg(52)
	diffFgRed   = fg(210)
	diffBgGreen = bg(22)
	diffFgGreen = fg(156)
	diffGutter  = fg(240)
)

var splashColorCode = 99

var (
	splashFrame   lipgloss.Style
	splashTitle   lipgloss.Style
	splashTagline = lipgloss.NewStyle().Italic(true)
)

func init() { rebuildSplashStyles() }

func rebuildSplashStyles() {
	col := lipgloss.Color(strconv.Itoa(splashColorCode))
	splashFrame = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(col).
		Padding(0, 1)
	splashTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(col)
}

func ApplyColors(overrides map[string]int) error {
	setters := map[string]func(int){
		"lavender_indigo": func(c int) { LavenderIndigo = fg(c) },
		"mauve":           func(c int) { Mauve = fg(c) },
		"blueberry":       func(c int) { Blueberry = fg(c) },
		"medium_purple":   func(c int) { MediumPurple = fg(c) },
		"bright_lavender": func(c int) { BrightLavender = fg(c) },
		"dark_violet":     func(c int) { DarkViolet = fg(c) },
		"pure_violet":     func(c int) { PureViolet = fg(c) },
		"electric_purple": func(c int) { ElectricPurple = fg(c) },
		"gray":            func(c int) { Gray = fg(c) },
		"white":           func(c int) { White = fg(c) },
		"red":             func(c int) { Red = fg(c) },
		"green":           func(c int) { Green = fg(c) },
		"pure_orange":     func(c int) { PureOrange = fg(c) },
		"diff_add_fg":     func(c int) { diffFgGreen = fg(c) },
		"diff_add_bg":     func(c int) { diffBgGreen = bg(c) },
		"diff_del_fg":     func(c int) { diffFgRed = fg(c) },
		"diff_del_bg":     func(c int) { diffBgRed = bg(c) },
		"diff_gutter":     func(c int) { diffGutter = fg(c) },
		"splash":          func(c int) { splashColorCode = c },
	}
	for key, code := range overrides {
		set, ok := setters[key]
		if !ok {
			return fmt.Errorf("unknown style color %q", key)
		}
		if code < 0 || code > 255 {
			return fmt.Errorf("style color %q out of range (want 0-255, got %d)", key, code)
		}
		set(code)
	}
	rebuildSplashStyles()
	return nil
}

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
