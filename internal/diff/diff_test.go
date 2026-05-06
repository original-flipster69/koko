package diff

import (
	"strings"
	"testing"
)

func TestUnified(t *testing.T) {
	tests := []struct {
		name     string
		oldText  string
		newText  string
		path     string
		expected string
	}{
		{
			name:     "no changes identical inputs",
			oldText:  "line1\nline2\nline3",
			newText:  "line1\nline2\nline3",
			path:     "file.go",
			expected: "",
		},
		{
			name:    "single line change with context",
			oldText: "line1\nline2\nline3\nline4\nline5\nline6\nline7",
			newText: "line1\nline2\nline3\nline4-modified\nline5\nline6\nline7",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,7 +1,7 @@",
				" line1",
				" line2",
				" line3",
				"-line4",
				"+line4-modified",
				" line5",
				" line6",
				" line7",
				"",
			}, "\n"),
		},
		{
			name:    "addition at end",
			oldText: "line1\nline2\nline3",
			newText: "line1\nline2\nline3\nline4\nline5",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,3 +1,5 @@",
				" line1",
				" line2",
				" line3",
				"+line4",
				"+line5",
				"",
			}, "\n"),
		},
		{
			name:    "deletion",
			oldText: "line1\nline2\nline3\nline4\nline5",
			newText: "line1\nline2\nline5",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,5 +1,3 @@",
				" line1",
				" line2",
				"-line3",
				"-line4",
				" line5",
				"",
			}, "\n"),
		},
		{
			name: "multiple hunks separated by more than 6 context lines",
			oldText: strings.Join([]string{
				"line1", "line2", "line3", "line4", "line5",
				"line6", "line7", "line8", "line9", "line10",
				"line11", "line12", "line13", "line14", "line15",
			}, "\n"),
			newText: strings.Join([]string{
				"line1-changed", "line2", "line3", "line4", "line5",
				"line6", "line7", "line8", "line9", "line10",
				"line11", "line12", "line13", "line14", "line15-changed",
			}, "\n"),
			path: "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,4 +1,4 @@",
				"-line1",
				"+line1-changed",
				" line2",
				" line3",
				" line4",
				"@@ -12,4 +12,4 @@",
				" line12",
				" line13",
				" line14",
				"-line15",
				"+line15-changed",
				"",
			}, "\n"),
		},
		{
			name: "adjacent changes within 6 lines merged into one hunk",
			oldText: strings.Join([]string{
				"line1", "line2", "line3", "line4", "line5",
				"line6", "line7", "line8", "line9",
			}, "\n"),
			newText: strings.Join([]string{
				"line1-changed", "line2", "line3", "line4", "line5",
				"line6", "line7-changed", "line8", "line9",
			}, "\n"),
			path: "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,9 +1,9 @@",
				"-line1",
				"+line1-changed",
				" line2",
				" line3",
				" line4",
				" line5",
				" line6",
				"-line7",
				"+line7-changed",
				" line8",
				" line9",
				"",
			}, "\n"),
		},
		{
			name:    "change at start of file less than 3 leading context lines",
			oldText: "line1\nline2\nline3\nline4",
			newText: "line1-changed\nline2\nline3\nline4",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,4 +1,4 @@",
				"-line1",
				"+line1-changed",
				" line2",
				" line3",
				" line4",
				"",
			}, "\n"),
		},
		{
			name:    "change at end of file less than 3 trailing context lines",
			oldText: "line1\nline2\nline3\nline4",
			newText: "line1\nline2\nline3\nline4-changed",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,4 +1,4 @@",
				" line1",
				" line2",
				" line3",
				"-line4",
				"+line4-changed",
				"",
			}, "\n"),
		},
		{
			name:    "empty old text entire file is new",
			oldText: "",
			newText: "line1\nline2\nline3",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,0 +1,3 @@",
				"+line1",
				"+line2",
				"+line3",
				"",
			}, "\n"),
		},
		{
			name:    "empty new text file deleted",
			oldText: "line1\nline2\nline3",
			newText: "",
			path:    "file.go",
			expected: strings.Join([]string{
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,3 +1,0 @@",
				"-line1",
				"-line2",
				"-line3",
				"",
			}, "\n"),
		},
		{
			name: "large context gap hunks do not balloon",
			oldText: func() string {
				lines := make([]string, 30)
				for i := range lines {
					lines[i] = "line" + strings.Repeat("x", i)
				}
				lines[2] = "changed-old-a"
				lines[27] = "changed-old-b"
				return strings.Join(lines, "\n")
			}(),
			newText: func() string {
				lines := make([]string, 30)
				for i := range lines {
					lines[i] = "line" + strings.Repeat("x", i)
				}
				lines[2] = "changed-new-a"
				lines[27] = "changed-new-b"
				return strings.Join(lines, "\n")
			}(),
			path: "file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Unified(tt.oldText, tt.newText, tt.path)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("unexpected output\nwant:\n%s\ngot:\n%s", tt.expected, result)
				}
				return
			}

			if tt.name == "no changes identical inputs" {
				if result != "" {
					t.Errorf("expected empty string for identical inputs, got:\n%s", result)
				}
				return
			}

			if tt.name == "large context gap hunks do not balloon" {
				verifyLargeContextGap(t, result)
				return
			}
		})
	}
}

func verifyLargeContextGap(t *testing.T, result string) {
	t.Helper()

	if result == "" {
		t.Fatal("expected non-empty diff for large context gap test")
	}

	if !strings.HasPrefix(result, "--- a/file.go\n+++ b/file.go\n") {
		t.Error("missing or incorrect file header")
	}

	hunkCount := strings.Count(result, "@@")
	if hunkCount < 4 {
		t.Errorf("expected at least 2 hunks (4 @@ markers), got %d @@ markers", hunkCount)
	}

	lines := strings.Split(result, "\n")
	contextCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, " ") {
			contextCount++
		}
	}

	if contextCount > 12 {
		t.Errorf("context lines should not balloon; got %d context lines (max expected ~12)", contextCount)
	}
}

func TestUnifiedHeaders(t *testing.T) {
	result := Unified("old\nline", "new\nline", "path/to/file.txt")

	if !strings.HasPrefix(result, "--- a/path/to/file.txt\n+++ b/path/to/file.txt\n") {
		t.Errorf("incorrect header format:\n%s", result)
	}
}

func TestUnifiedHunkLineNumbers(t *testing.T) {
	oldText := strings.Join([]string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
	}, "\n")
	newText := strings.Join([]string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j-modified",
	}, "\n")

	result := Unified(oldText, newText, "test.go")

	if !strings.Contains(result, "@@ -7,4 +7,4 @@") {
		t.Errorf("expected hunk header @@ -7,4 +7,4 @@, got:\n%s", result)
	}
}
