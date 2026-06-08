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

const (
	LavenderIndigo = "\033[38;5;135m"
	Mauve          = "\033[38;5;183m"
	Blueberry      = "\033[38;5;99m"
	MediumPurple   = "\033[38;5;98m"
	BrightLavender = "\033[38;5;141m"
	DarkViolet     = "\033[38;5;55m"
	PureViolet     = "\033[38;5;93m"
	ElectricPurple = "\033[38;5;129m"
	Gray           = "\033[38;5;243m"
	White          = "\033[38;5;255m"
	Red            = "\033[38;5;197m"
	Green          = "\033[38;5;114m"
	PureOrange     = "\033[38;5;214m"
)

var (
	Primary   = Blueberry
	Secondary = LavenderIndigo
	Highlight = Mauve
	Label     = BrightLavender
	Value     = White
	Muted     = Gray
	Danger    = Red
	Success   = Green
	Code      = PureOrange

	DiffAddFg  = "\033[38;5;156m"
	DiffAddBg  = "\033[48;5;22m"
	DiffDelFg  = "\033[38;5;210m"
	DiffDelBg  = "\033[48;5;52m"
	DiffGutter = "\033[38;5;240m"
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
		"primary":     func(c int) { Primary = fg(c) },
		"secondary":   func(c int) { Secondary = fg(c) },
		"highlight":   func(c int) { Highlight = fg(c) },
		"label":       func(c int) { Label = fg(c) },
		"value":       func(c int) { Value = fg(c) },
		"muted":       func(c int) { Muted = fg(c) },
		"error":       func(c int) { Danger = fg(c) },
		"success":     func(c int) { Success = fg(c) },
		"code":        func(c int) { Code = fg(c) },
		"diff_add_fg": func(c int) { DiffAddFg = fg(c) },
		"diff_add_bg": func(c int) { DiffAddBg = bg(c) },
		"diff_del_fg": func(c int) { DiffDelFg = fg(c) },
		"diff_del_bg": func(c int) { DiffDelBg = bg(c) },
		"diff_gutter": func(c int) { DiffGutter = fg(c) },
		"splash":      func(c int) { splashColorCode = c },
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
	return fmt.Sprintf("  %s%-9s%s %s%s%s", Bold+Label, label, Reset, Value, value, Reset)
}

func Error(text string) string {
	return fmt.Sprintf("%s%serror:%s %s", Bold, Danger, Reset, text)
}

func TokenStats(input, output int) string {
	return fmt.Sprintf("  %s%stokens: %d in / %d out%s", Dim, Muted, input, output, Reset)
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
				out.WriteString(fmt.Sprintf("  %s%s╭─ %s%s\n", Bold, Primary, path, Reset))
				headerPrinted = true
			}
			continue
		case strings.HasPrefix(line, "@@"):
			var oc, nc int
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldLine, &oc, &newLine, &nc)
			out.WriteString(fmt.Sprintf("  %s│ %s%s%s\n", Primary, Dim+Muted, line, Reset))
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
			bg, fg, prefix = DiffDelBg, DiffDelFg, " - "
			oldLine++
		case "+":
			gutter = fmt.Sprintf("     %4d", newLine)
			bg, fg, prefix = DiffAddBg, DiffAddFg, " + "
			newLine++
		default:
			gutter = fmt.Sprintf("%4d %4d", oldLine, newLine)
			bg, fg, prefix = "", Muted, "   "
			oldLine++
			newLine++
		}

		if bg != "" {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s%s\n",
				Primary, DiffGutter, gutter, bg, fg, prefix, content, Reset))
		} else {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s\n",
				Primary, DiffGutter, gutter, fg, prefix, content, Reset))
		}
	}

	if headerPrinted {
		out.WriteString(fmt.Sprintf("  %s╰─%s\n", Primary, Reset))
	}
	return out.String()
}
