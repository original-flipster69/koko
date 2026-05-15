package ui

import "strings"

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
	cur := ""
	for _, c := range line {
		var color string
		switch c {
		case '█':
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
