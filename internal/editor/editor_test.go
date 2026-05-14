package editor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/editor"
	"github.com/original-flipster69/koko/internal/sandbox"
)

func setup(t *testing.T) (string, *editor.Editor, *sandbox.Sandbox) {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := sandbox.New(resolved, []string{resolved}, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	return resolved, editor.New(sb), sb
}

func vp(t *testing.T, sb *sandbox.Sandbox, p string) sandbox.ValidPath {
	t.Helper()
	v, err := sb.ValidatePath(p)
	if err != nil {
		t.Fatalf("validate %q: %v", p, err)
	}
	return v
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0664); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceInFile(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		oldText   string
		newText   string
		wantAfter string
		wantErr   string
	}{
		{
			name:      "exact match single occurrence",
			initial:   "hello world\nfoo bar\n",
			oldText:   "foo bar",
			newText:   "baz qux",
			wantAfter: "hello world\nbaz qux\n",
		},
		{
			name:      "exact match replaces only first of one",
			initial:   "aaa\nbbb\nccc\n",
			oldText:   "bbb",
			newText:   "BBB",
			wantAfter: "aaa\nBBB\nccc\n",
		},
		{
			name:    "multiple exact matches returns error",
			initial: "foo\nfoo\nbar\n",
			oldText: "foo",
			newText: "replaced",
			wantErr: "appears 2 times",
		},
		{
			name:    "not found returns error",
			initial: "hello world\n",
			oldText: "does not exist",
			newText: "x",
			wantErr: "old_text not found",
		},
		{
			name:      "fuzzy whitespace match tabs vs spaces",
			initial:   "func main() {\n\tfmt.Println(\"hi\")\n}\n",
			oldText:   "func main() {\n  fmt.Println(\"hi\")\n}",
			newText:   "func main() {\n\tfmt.Println(\"bye\")\n}",
			wantAfter: "func main() {\n\tfmt.Println(\"bye\")\n}\n",
		},
		{
			name:      "fuzzy whitespace match CRLF vs LF",
			initial:   "line1\nline2\nline3\n",
			oldText:   "line1\r\nline2\r\nline3",
			newText:   "A\nB\nC",
			wantAfter: "A\nB\nC\n",
		},
		{
			name:      "trimmed line match handles indentation differences",
			initial:   "  if true {\n    doStuff()\n  }\n",
			oldText:   "if true {\ndoStuff()\n}",
			newText:   "if false {\n    skip()\n  }",
			wantAfter: "  if false {\n    skip()\n  }\n",
		},
		{
			name:    "trimmed line match ambiguous multiple",
			initial: "  x := 1\n  x := 1\n",
			oldText: "x := 1",
			newText: "x := 2",
			wantErr: "ambiguous",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := filepath.Join(tmpDir, "test.txt")
			writeTestFile(t, path, tc.initial)

			ed.MarkRead(vp(t, sb, path), tc.initial)

			_, after, err := ed.ReplaceInFile(vp(t, sb, path), tc.oldText, tc.newText)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if after != tc.wantAfter {
				t.Fatalf("after mismatch:\n  got:  %q\n  want: %q", after, tc.wantAfter)
			}

			ondisk, _ := os.ReadFile(path)
			if string(ondisk) != tc.wantAfter {
				t.Fatalf("on-disk content mismatch:\n  got:  %q\n  want: %q", string(ondisk), tc.wantAfter)
			}
		})
	}
}

func TestReadBeforeEditEnforcement(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string
		wantErr string
	}{
		{
			name: "refuses if file not read",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "unread.txt")
				writeTestFile(t, path, "content")
				return path
			},
			wantErr: "file has not been read",
		},
		{
			name: "refuses if file changed on disk after read",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "changed.txt")
				writeTestFile(t, path, "original")
				ed.MarkRead(vp(t, sb, path), "original")
				writeTestFile(t, path, "modified behind our back")
				return path
			},
			wantErr: "file content changed on disk",
		},
		{
			name: "succeeds after MarkRead",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "good.txt")
				writeTestFile(t, path, "aaa bbb ccc")
				ed.MarkRead(vp(t, sb, path), "aaa bbb ccc")
				return path
			},
			wantErr: "",
		},
		{
			name: "hash updated after successful replace allows next replace",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "multi.txt")
				writeTestFile(t, path, "first line\nsecond line\n")
				ed.MarkRead(vp(t, sb, path), "first line\nsecond line\n")
				_, _, err := ed.ReplaceInFile(vp(t, sb, path), "first line", "1st line")
				if err != nil {
					t.Fatalf("first replace failed: %v", err)
				}
				_, _, err = ed.ReplaceInFile(vp(t, sb, path), "second line", "2nd line")
				if err != nil {
					t.Fatalf("second replace failed: %v", err)
				}
				return path
			},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := tc.setup(t, tmpDir, ed, sb)

			if tc.name == "succeeds after MarkRead" {
				_, _, err := ed.ReplaceInFile(vp(t, sb, path), "bbb", "BBB")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if tc.name == "hash updated after successful replace allows next replace" {
				return
			}

			_, _, err := ed.ReplaceInFile(vp(t, sb, path), "content", "new")
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriteFileUpdatesHash(t *testing.T) {
	tmpDir, ed, sb := setup(t)
	path := filepath.Join(tmpDir, "written.txt")

	err := ed.WriteFile(vp(t, sb, path), "initial content", false)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, _, err = ed.ReplaceInFile(vp(t, sb, path), "initial", "updated")
	if err != nil {
		t.Fatalf("ReplaceInFile after WriteFile should succeed: %v", err)
	}
}

func TestLockfileGuard(t *testing.T) {
	lockfiles := []string{
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"go.sum",
		"Cargo.lock",
		"poetry.lock",
		"Pipfile.lock",
		"Gemfile.lock",
		"composer.lock",
		"mix.lock",
		"pdm.lock",
		"uv.lock",
	}

	for _, lf := range lockfiles {
		t.Run("WriteFile/"+lf, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := filepath.Join(tmpDir, lf)
			err := ed.WriteFile(vp(t, sb, path), "data", false)
			if err == nil {
				t.Fatal("expected lockfile error, got nil")
			}
			if !strings.Contains(err.Error(), "refusing to modify lockfile") {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("ReplaceInFile/"+lf, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := filepath.Join(tmpDir, lf)
			writeTestFile(t, path, "data")
			ed.MarkRead(vp(t, sb, path), "data")
			_, _, err := ed.ReplaceInFile(vp(t, sb, path), "data", "new")
			if err == nil {
				t.Fatal("expected lockfile error, got nil")
			}
			if !strings.Contains(err.Error(), "refusing to modify lockfile") {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		t.Run("DeleteFile/"+lf, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := filepath.Join(tmpDir, lf)
			writeTestFile(t, path, "data")
			err := ed.DeleteFile(vp(t, sb, path))
			if err == nil {
				t.Fatal("expected lockfile error, got nil")
			}
			if !strings.Contains(err.Error(), "refusing to modify lockfile") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name      string
		existing  bool
		overwrite bool
		wantErr   string
	}{
		{
			name:      "creates new file",
			existing:  false,
			overwrite: false,
			wantErr:   "",
		},
		{
			name:      "refuses overwrite when false",
			existing:  true,
			overwrite: false,
			wantErr:   "refusing to write",
		},
		{
			name:      "allows overwrite when true",
			existing:  true,
			overwrite: true,
			wantErr:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := filepath.Join(tmpDir, "file.txt")

			if tc.existing {
				writeTestFile(t, path, "existing content")
			}

			err := ed.WriteFile(vp(t, sb, path), "new content", tc.overwrite)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			data, _ := os.ReadFile(path)
			if string(data) != "new content" {
				t.Fatalf("file content mismatch: got %q", string(data))
			}
		})
	}
}

func TestUndo(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string
		wantPath   string
		wantOnDisk string
		wantGone   bool
	}{
		{
			name: "undo restores previous content",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "undo.txt")
				writeTestFile(t, path, "before")
				ed.MarkRead(vp(t, sb, path), "before")
				_, _, err := ed.ReplaceInFile(vp(t, sb, path), "before", "after")
				if err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantOnDisk: "before",
		},
		{
			name: "undo of new file deletes it",
			setup: func(t *testing.T, tmpDir string, ed *editor.Editor, sb *sandbox.Sandbox) string {
				path := filepath.Join(tmpDir, "brand_new.txt")
				if err := ed.WriteFile(vp(t, sb, path), "created", false); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantGone: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, ed, sb := setup(t)
			path := tc.setup(t, tmpDir, ed, sb)

			undone, err := ed.Undo()
			if err != nil {
				t.Fatalf("undo failed: %v", err)
			}
			if undone != path {
				t.Fatalf("undo returned path %q, want %q", undone, path)
			}

			if tc.wantGone {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("expected file to be deleted after undo")
				}
				return
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading file after undo: %v", err)
			}
			if string(data) != tc.wantOnDisk {
				t.Fatalf("after undo content = %q, want %q", string(data), tc.wantOnDisk)
			}
		})
	}
}

func TestUndoEmptyStack(t *testing.T) {
	_, ed, _ := setup(t)
	path, err := ed.Undo()
	if err != nil {
		t.Fatalf("undo on empty stack should not error: %v", err)
	}
	if path != "" {
		t.Fatalf("undo on empty stack should return empty string, got %q", path)
	}
}
