package ui

import "strings"

var mascotFrame1 = []string{
	`                 /▇▇▇\`,
	`               /[▇▇▇▇▇]\`,
	`              /[▇▇▇▇▇▇▇]\`,
	`             ██████████|\_`,
	`            _ █ ██ ███(▇▇|`,
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
	`            _ █ █ ███(▇▇|`,
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
	`            _ █ █ ███(▇▇|`,
	`        ╭∩╮ /██████|▇/▇▇\_`,
	`         (▇▇[██████]▇▇/(▇)\_`,
	`          |▇▇▇)▇▇▇▇/▇▇[▇▇▇)|`,
	`                 /▇▇▇▇▇]▇_▇|`,
	`                    ████| |]`,
}

func (s Scheme) Mascot() string {
	return s.renderMascot(mascotFrame1)
}

func (s Scheme) MascotFrames() []string {
	return []string{
		s.renderMascot(mascotFrame1),
		s.renderMascot(mascotFrame2),
		s.renderMascot(mascotFrame3),
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

func (s Scheme) renderMascot(raw []string) string {
	var b strings.Builder
	for _, line := range raw {
		if pad := mascotWidth - len([]rune(line)); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(s.colorizeMascot(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func (s Scheme) colorizeMascot(line string) string {
	var b strings.Builder
	cur := ""
	for _, c := range line {
		var color string
		switch c {
		case '█', '╭', '∩', '╮', '❤':
			color = s.Highlight
		case '▇':
			color = s.Accent
		case ' ':
			color = Reset
		default:
			color = s.Primary
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
