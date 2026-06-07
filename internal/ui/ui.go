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
	LavenderIndigo = 135
	Mauve          = 183
	Blueberry      = 99
	MediumPurple   = 98
	BrightLavender = 141
	DarkViolet     = 55
	PureViolet     = 93
	ElectricPurple = 129
	Gray           = 243
	White          = 255
	Red            = 197
	Green          = 114
	PureOrange     = 214
)

//FIXME potentially encapsulate the whole UI handling in a separate thing that then has the color scheme inside... but ok for now

type Scheme struct {
	Primary    string
	Secondary  string
	Highlight  string
	Accent     string
	Label      string
	Value      string
	Muted      string
	Danger     string
	Success    string
	Code       string
	DiffAddFg  string
	DiffAddBg  string
	DiffDelFg  string
	DiffDelBg  string
	DiffGutter string
	Splash     int
}

func DefaultScheme() Scheme {
	return Scheme{
		Primary:    fg(Blueberry),
		Secondary:  fg(LavenderIndigo),
		Highlight:  fg(Mauve),
		Accent:     fg(DarkViolet),
		Label:      fg(BrightLavender),
		Value:      fg(White),
		Muted:      fg(Gray),
		Danger:     fg(Red),
		Success:    fg(Green),
		Code:       fg(PureOrange),
		DiffAddFg:  fg(156),
		DiffAddBg:  bg(22),
		DiffDelFg:  fg(210),
		DiffDelBg:  bg(52),
		DiffGutter: fg(240),
		Splash:     Blueberry,
	}
}

func (s Scheme) With(overrides map[string]int) (Scheme, error) {
	fgRoles := map[string]*string{
		"primary": &s.Primary, "secondary": &s.Secondary, "highlight": &s.Highlight,
		"accent": &s.Accent,
		"label":  &s.Label, "value": &s.Value, "muted": &s.Muted,
		"error": &s.Danger, "success": &s.Success, "code": &s.Code,
		"diff_add_fg": &s.DiffAddFg, "diff_del_fg": &s.DiffDelFg, "diff_gutter": &s.DiffGutter,
	}
	bgRoles := map[string]*string{
		"diff_add_bg": &s.DiffAddBg, "diff_del_bg": &s.DiffDelBg,
	}
	for key, code := range overrides {
		if code < 0 || code > 255 {
			return s, fmt.Errorf("style color %q out of range (want 0-255, got %d)", key, code)
		}
		switch {
		case key == "splash":
			s.Splash = code
		case fgRoles[key] != nil:
			*fgRoles[key] = fg(code)
		case bgRoles[key] != nil:
			*bgRoles[key] = bg(code)
		default:
			return s, fmt.Errorf("unknown style color %q", key)
		}
	}
	return s, nil
}

func (s Scheme) Splashscreen(mascot, provider, model, sandbox, version string, detected []string) string {
	col := lipgloss.Color(strconv.Itoa(s.Splash))
	frame := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(col).Padding(0, 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(col)
	tagline := lipgloss.NewStyle().Italic(true)

	left := strings.TrimRight(mascot, "\n")
	rightLines := []string{
		"",
		title.Render(" k o k o "),
		tagline.Render("  secure coding assistant"),
		"",
		s.Info("version ", version),
		s.Info("provider", provider),
		s.Info("model   ", model),
		s.Info("sandbox ", sandbox),
	}
	if len(detected) > 0 {
		rightLines = append(rightLines, s.Info("stack", strings.Join(detected, ", ")))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Center, left, "    ", strings.Join(rightLines, "\n"))
	return frame.Render(body) + "\n"
}

func (s Scheme) Info(label string, value string) string {
	return fmt.Sprintf("  %s%-9s%s %s%s%s", Bold+s.Label, label, Reset, s.Value, value, Reset)
}

func (s Scheme) Error(text string) string {
	return fmt.Sprintf("%s%serror:%s %s", Bold, s.Danger, Reset, text)
}

func (s Scheme) TokenStats(input, output int) string {
	return fmt.Sprintf("  %s%stokens: %d in / %d out%s", Dim, s.Muted, input, output, Reset)
}

func PrivacyWarning(providerName string) string {
	if providerName == "ollama" {
		return ""
	}
	return fmt.Sprintf("  %s%s⚠ privacy:%s %sanything you send is processed by a remote provider (%s) and may be retained by it. Avoid sharing secrets or sensitive data — use Ollama for fully local, private inference.%s",
		Bold, fg(PureOrange), Reset, fg(PureOrange), providerName, Reset)
}

func (s Scheme) ColorDiff(diffText string) string {
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
				out.WriteString(fmt.Sprintf("  %s%s╭─ %s%s\n", Bold, s.Primary, path, Reset))
				headerPrinted = true
			}
			continue
		case strings.HasPrefix(line, "@@"):
			var oc, nc int
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldLine, &oc, &newLine, &nc)
			out.WriteString(fmt.Sprintf("  %s│ %s%s%s\n", s.Primary, Dim+s.Muted, line, Reset))
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
			bg, fg, prefix = s.DiffDelBg, s.DiffDelFg, " - "
			oldLine++
		case "+":
			gutter = fmt.Sprintf("     %4d", newLine)
			bg, fg, prefix = s.DiffAddBg, s.DiffAddFg, " + "
			newLine++
		default:
			gutter = fmt.Sprintf("%4d %4d", oldLine, newLine)
			bg, fg, prefix = "", s.Muted, "   "
			oldLine++
			newLine++
		}

		if bg != "" {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s%s\n",
				s.Primary, s.DiffGutter, gutter, bg, fg, prefix, content, Reset))
		} else {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s\n",
				s.Primary, s.DiffGutter, gutter, fg, prefix, content, Reset))
		}
	}

	if headerPrinted {
		out.WriteString(fmt.Sprintf("  %s╰─%s\n", s.Primary, Reset))
	}
	return out.String()
}
