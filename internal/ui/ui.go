package ui

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	Reset         = "\033[0m"
	Bold          = "\033[1m"
	Dim           = "\033[2m"
	Italic        = "\033[3m"
	Underline     = "\033[4m"
	Strikethrough = "\033[9m"
	Purple        = "\033[38;5;135m"
	LightPurp     = "\033[38;5;183m"
	BrightPurp    = "\033[38;5;99m"
	DarkPurp      = "\033[38;5;54m"
	Violet        = "\033[38;5;141m"
	Gray          = "\033[38;5;243m"
	White         = "\033[38;5;255m"
	BgPurple      = "\033[48;5;53m"

	DKBrown  = "\033[38;5;94m"
	DKTan    = "\033[38;5;223m"
	DKMuzzle = "\033[38;5;180m"
	DKRed    = "\033[38;5;196m"
	DKYellow = "\033[38;5;226m"
	DKBlack  = "\033[38;5;16m"
)

func Mascot() string {
	raw := []string{
		`                 /▇▇▇\`,
		`               /[▇▇▇▇▇]\`,
		`              /[▇▇▇▇▇▇▇]\`,
		`             ██████████|\_`,
		`            _ ██ █ ███(▇▇|`,
		`           _/███████(▇▇)▇|_`,
		`           /▇███████|▇▇/▇▇)\_`,
		`           |▇▇\▇▇▇▇/▇▇/(▇▇▇▇)`,
		`           |▇▇▇/  \▇▇/ |▇▇▇▇|`,
		`            ████  ████   \| ]`,
	}
	var b strings.Builder
	for _, line := range raw {
		b.WriteString(colorizeMascot(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func colorizeMascot(line string) string {
	var b strings.Builder
	cur := byte(0)
	for _, c := range line {
		var cat byte
		switch c {
		case '█':
			cat = 'F'
		case '▇':
			cat = 'L'
		case ' ':
			cat = 'S'
		default:
			cat = 'O'
		}
		if cat != cur {
			switch cat {
			case 'F':
				b.WriteString(LightPurp)
			case 'L':
				b.WriteString(DarkPurp)
			case 'O':
				b.WriteString(BrightPurp)
			case 'S':
				b.WriteString(Reset)
			}
			cur = cat
		}
		b.WriteRune(c)
	}
	b.WriteString(Reset)
	return b.String()
}

func Banner() string {
	return fmt.Sprintf(""+
		"%s%s╔══════════════════════════════════════════╗%s\n"+
		"%s%s║%s  %s%s k o k o%s                        %s%s║%s\n"+
		"%s%s║%s  %s▸ secure coding assistant%s               %s%s║%s\n"+
		"%s%s╚══════════════════════════════════════════╝%s",
		Bold, BrightPurp, Reset,
		Bold, BrightPurp, Reset, Bold, LightPurp, Reset, Bold, BrightPurp, Reset,
		Bold, BrightPurp, Reset, Purple, Reset, Bold, BrightPurp, Reset,
		Bold, BrightPurp, Reset,
	)
}

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

	title := fmt.Sprintf("%s%s k o k o %s", Bold, BrightPurp, Reset)
	tagline := fmt.Sprintf("%s▸ secure coding assistant%s", Purple, Reset)

	var right []string
	right = append(right, "")
	right = append(right, title)
	right = append(right, tagline)
	right = append(right, "")
	if version != "" {
		right = append(right, Info("version ", version))
	}
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
	out.WriteString(Bold + BrightPurp + "╔" + strings.Repeat("═", totalW) + "╗" + Reset + "\n")
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
		out.WriteString(Bold + BrightPurp + "║" + Reset + innerPadL + l + lPad + gap + r + rPad + innerPadR + Bold + BrightPurp + "║" + Reset + "\n")
	}
	out.WriteString(Bold + BrightPurp + "╚" + strings.Repeat("═", totalW) + "╝" + Reset + "\n")
	return out.String()
}

func Info(label string, value string) string {
	return fmt.Sprintf("  %s%-9s%s %s%s%s", DarkPurp, label, Reset, Violet, value, Reset)
}

func Prompt() string {
	bar := strings.Repeat("─", 40)
	return fmt.Sprintf("%s%s╭─ you %s%s\n%s%s│ ▶ %s",
		Bold, BrightPurp, bar, Reset,
		Bold, BrightPurp, Reset,
	)
}

func MultilinePrompt() string {
	return fmt.Sprintf("%s%s · %s", Dim, Purple, Reset)
}

var toolSymbols = map[string]string{
	"read_file":       "◇",
	"write_file":      "✎",
	"replace_in_file": "✎",
	"delete_file":     "✕",
	"rename_file":     "⇄",
	"list_dir":        "≡",
	"search_files":    "⌕",
	"exec_command":    "⚡",
	"save_memory":     "◆",
	"delete_memory":   "◆",
	"list_memories":   "◆",
}

func ToolTag(name string) string {
	sym := "▪"
	if s, ok := toolSymbols[name]; ok {
		sym = s
	}
	return fmt.Sprintf("%s%s%s%s", Bold, Purple, sym, Reset)
}

func FormatToolResult(name string, result string) string {
	if strings.HasPrefix(result, "error:") {
		return fmt.Sprintf("%s\n  %s%s%s", ToolTag(name), Red, result, Reset)
	}
	return fmt.Sprintf("%s %s%s%s", ToolTag(name), LightPurp, result, Reset)
}

func Error(text string) string {
	return fmt.Sprintf("%s%serror:%s %s", Bold, "\033[38;5;197m", Reset, text)
}

func TokenStats(input, output int) string {
	return fmt.Sprintf("  %s%stokens: %d in / %d out%s", Dim, Gray, input, output, Reset)
}

var goodbyeLines = []string{
	"see you later, space cowboy",
	"off to file a bug report with the universe",
	"don't touch the repo while I'm gone",
	"my circuits need a nap",
	"ctrl+c'd back to the shadow realm",
	"ok but who's going to refactor this while I'm away",
	"going to the banana farm, brb",
	"tell my children (goroutines) I loved them",
	"closing stream, opening beer",
	"may your diffs be small and your builds be green",
	"commit early, commit often, but not now — i'm leaving",
	"rm -rf /me",
	"signing off — try not to push to main",
	"I was a good gorilla, right?",
	"exit 0, for once",
	"see you in the next session, legend",
	"logging off — don't let the tests see me go",
	"poof",
	"banana break. call me if anything catches fire",
	"auf wiedersehen, build warriors",
}

func Goodbye() string {
	line := goodbyeLines[rand.Intn(len(goodbyeLines))]
	return fmt.Sprintf("\n%s%s  ✦ %s %s", Dim, Purple, line, Reset)
}

const (
	Red   = "\033[38;5;197m"
	Green = "\033[38;5;114m"
	Amber = "\033[38;5;214m"
)

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
				out.WriteString(fmt.Sprintf("  %s%s╭─ %s%s\n", Bold, BrightPurp, path, Reset))
				headerPrinted = true
			}
			continue
		case strings.HasPrefix(line, "@@"):
			var oc, nc int
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldLine, &oc, &newLine, &nc)
			out.WriteString(fmt.Sprintf("  %s│ %s%s%s\n", BrightPurp, Dim+Gray, line, Reset))
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
				BrightPurp, diffGutter, gutter, bg, fg, prefix, content, Reset))
		} else {
			out.WriteString(fmt.Sprintf("  %s│ %s%s %s%s%s%s\n",
				BrightPurp, diffGutter, gutter, fg, prefix, content, Reset))
		}
	}

	if headerPrinted {
		out.WriteString(fmt.Sprintf("  %s╰─%s\n", BrightPurp, Reset))
	}
	return out.String()
}

func diffWidth() int {
	if c := termCols(); c >= 40 {
		return c
	}
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 40 {
			return n
		}
	}
	return 100
}

func termCols() int {
	type winsize struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	ws := &winsize{}
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if err != 0 || ws.Col == 0 {
		return 0
	}
	return int(ws.Col)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
