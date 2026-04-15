package diff

import (
	"fmt"
	"strings"
)

func Unified(oldText, newText, path string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	lcs := longestCommonSubseq(oldLines, newLines)

	var hunks []hunk
	var current *hunk

	oi, ni, li := 0, 0, 0
	for oi < len(oldLines) || ni < len(newLines) {
		if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) &&
			oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
			if current != nil {
				current.lines = append(current.lines, " "+oldLines[oi])
			}
			oi++
			ni++
			li++
			continue
		}

		if current == nil {
			h := hunk{oldStart: oi + 1, newStart: ni + 1}
			contextStart := max(0, oi-3)
			for c := contextStart; c < oi; c++ {
				h.lines = append(h.lines, " "+oldLines[c])
			}
			if contextStart < oi {
				h.oldStart = contextStart + 1
				h.newStart = ni - (oi - contextStart) + 1
			}
			current = &h
		}

		if li < len(lcs) && oi < len(oldLines) && oldLines[oi] != lcs[li] {
			current.lines = append(current.lines, "-"+oldLines[oi])
			oi++
			continue
		}
		if li < len(lcs) && ni < len(newLines) && newLines[ni] != lcs[li] {
			current.lines = append(current.lines, "+"+newLines[ni])
			ni++
			continue
		}
		if oi >= len(oldLines) && ni < len(newLines) {
			current.lines = append(current.lines, "+"+newLines[ni])
			ni++
			continue
		}
		if ni >= len(newLines) && oi < len(oldLines) {
			current.lines = append(current.lines, "-"+oldLines[oi])
			oi++
			continue
		}
	}

	if current != nil {
		hunks = append(hunks, *current)
	}

	if len(hunks) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	for _, h := range hunks {
		oldCount, newCount := 0, 0
		for _, l := range h.lines {
			switch {
			case strings.HasPrefix(l, "-"):
				oldCount++
			case strings.HasPrefix(l, "+"):
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

type hunk struct {
	oldStart int
	newStart int
	lines    []string
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
