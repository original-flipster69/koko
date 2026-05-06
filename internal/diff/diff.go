package diff

import (
	"fmt"
	"strings"
)

const contextLines = 3

type editOp struct {
	kind byte
	line string
}

type hunk struct {
	oldStart int
	newStart int
	lines    []string
}

func Unified(oldText, newText, path string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	lcs := longestCommonSubseq(oldLines, newLines)

	ops := buildOps(oldLines, newLines, lcs)
	hunks := groupHunks(ops)
	if len(hunks) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	for _, h := range hunks {
		oldCount, newCount := 0, 0
		for _, l := range h.lines {
			switch l[0] {
			case '-':
				oldCount++
			case '+':
				newCount++
			default:
				oldCount++
				newCount++
			}
		}
		out.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.oldStart, oldCount, h.newStart, newCount))
		for _, l := range h.lines {
			out.WriteString(l + "\n")
		}
	}
	return out.String()
}

func buildOps(oldLines, newLines, lcs []string) []editOp {
	var ops []editOp
	oi, ni, li := 0, 0, 0
	for oi < len(oldLines) || ni < len(newLines) {
		if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) &&
			oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
			ops = append(ops, editOp{' ', oldLines[oi]})
			oi++
			ni++
			li++
			continue
		}
		if li < len(lcs) && oi < len(oldLines) && oldLines[oi] != lcs[li] {
			ops = append(ops, editOp{'-', oldLines[oi]})
			oi++
			continue
		}
		if li < len(lcs) && ni < len(newLines) && newLines[ni] != lcs[li] {
			ops = append(ops, editOp{'+', newLines[ni]})
			ni++
			continue
		}
		if oi < len(oldLines) {
			ops = append(ops, editOp{'-', oldLines[oi]})
			oi++
			continue
		}
		if ni < len(newLines) {
			ops = append(ops, editOp{'+', newLines[ni]})
			ni++
			continue
		}
	}
	return ops
}

func groupHunks(ops []editOp) []hunk {
	var hunks []hunk
	var cur *hunk
	var leadBuf []editOp
	oldLine, newLine := 1, 1
	trailing := 0
	gapAllowed := 2 * contextLines

	flush := func() {
		if cur == nil {
			return
		}
		if trailing > contextLines {
			excess := trailing - contextLines
			cur.lines = cur.lines[:len(cur.lines)-excess]
		}
		hunks = append(hunks, *cur)
		cur = nil
		trailing = 0
		leadBuf = nil
	}

	for _, o := range ops {
		if o.kind == ' ' {
			if cur == nil {
				leadBuf = append(leadBuf, o)
				if len(leadBuf) > contextLines {
					leadBuf = leadBuf[1:]
				}
			} else {
				cur.lines = append(cur.lines, " "+o.line)
				trailing++
				if trailing >= gapAllowed {
					flush()
				}
			}
			oldLine++
			newLine++
			continue
		}

		if cur == nil {
			h := hunk{}
			lead := len(leadBuf)
			h.oldStart = oldLine - lead
			h.newStart = newLine - lead
			if h.oldStart < 1 {
				h.oldStart = 1
			}
			if h.newStart < 1 {
				h.newStart = 1
			}
			for _, c := range leadBuf {
				h.lines = append(h.lines, " "+c.line)
			}
			leadBuf = nil
			cur = &h
		}
		cur.lines = append(cur.lines, string(o.kind)+o.line)
		trailing = 0

		switch o.kind {
		case '-':
			oldLine++
		case '+':
			newLine++
		}
	}
	flush()
	return hunks
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func longestCommonSubseq(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	result := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append(result, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	return result
}
