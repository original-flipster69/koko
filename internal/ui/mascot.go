package ui

import "strings"

var mascotFrame1 = []string{
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

var mascotFrame2 = []string{
	`                _____`,
	`              _/▇▇▇▇▇)\_`,
	`              /▇▇▇▇▇▇▇]|`,
	`             █████████/\_`,
	`            _ █ ██ ██(▇▇|`,
	`           //██████|▇/▇▇\_`,
	`           |[██████]▇▇/(▇)\_`,
	`           |▇▇)▇▇▇▇/▇▇[▇▇▇)|`,
	`            (▇▇)/ ▇▇▇▇▇]▇_▇|`,
	`              ████  ████| |]`,
}

var mascotFrame3 = []string{
	`                _____`,
	`              _/▇▇▇▇▇)\_`,
	`              /▇▇▇▇▇▇▇]|`,
	`             █████████/\_`,
	`            _ █ ██ ██(▇▇|`,
	`        ╭∩╮ /██████|▇/▇▇\_`,
	`         (▇▇[██████]▇▇/(▇)\_`,
	`          |▇▇▇)▇▇▇▇/▇▇[▇▇▇)|`,
	`            (▇▇)/ ▇▇▇▇▇]▇_▇|`,
	`              ████  ████| |]`,
}

func Mascot() string {
	return renderMascot(mascotFrame1)
}

func MascotFrames() []string {
	return []string{
		renderMascot(mascotFrame1),
		renderMascot(mascotFrame2),
		renderMascot(mascotFrame3),
	}
}

var mascotWidth = func() int {
	max := 0
	for _, frame := range [][]string{mascotFrame1, mascotFrame2, mascotFrame3} {
		for _, line := range frame {
			if w := len([]rune(line)); w > max {
				max = w
			}
		}
	}
	return max
}()

func renderMascot(raw []string) string {
	var b strings.Builder
	for _, line := range raw {
		if pad := mascotWidth - len([]rune(line)); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(colorizeMascot(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func colorizeMascot(line string) string {
	var b strings.Builder
	cur := ""
	for _, c := range line {
		var color string
		switch c {
		case '█', '╭', '∩', '╮', '❤':
			color = Mauve
		case '▇':
			color = DarkViolet
		case ' ':
			color = Reset
		default:
			color = Blueberry
		}
		if color != cur {
			b.WriteString(color)
			cur = color
		}
		b.WriteRune(c)
	}
	b.WriteString(Reset)
	return b.String()
}
