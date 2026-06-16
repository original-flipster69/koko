package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/editor"
	"github.com/original-flipster69/koko/internal/ignore"
	"github.com/original-flipster69/koko/internal/provider"
	"github.com/original-flipster69/koko/internal/sandbox"
)

func newListAgent(t *testing.T, root string, deny []string) *Agent {
	t.Helper()
	sb, err := sandbox.New(root, []string{root}, deny, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return &Agent{sandbox: sb, editor: editor.New(sb), ignore: ignore.NewFromPatterns(nil)}
}

func mkfile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListDirExcludesGitAndDenied(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "main.go")
	mkfile(t, root, ".env")        // denied
	mkfile(t, root, "secret.key")  // denied (*.key)
	mkfile(t, root, ".git/config") // inside .git → always denied

	a := newListAgent(t, root, []string{".env", "*.key"})
	out := a.listDir(context.Background(), provider.ToolCall{Args: map[string]string{"path": root}})

	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in listing, got:\n%s", out)
	}
	for _, hidden := range []string{".git", ".env", "secret.key"} {
		if strings.Contains(out, hidden) {
			t.Errorf("%q should not appear in list_dir output:\n%s", hidden, out)
		}
	}
}

func TestListDirTreeExcludesGit(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/app.go")
	mkfile(t, root, ".git/config")

	a := newListAgent(t, root, nil)
	out := a.listDir(context.Background(), provider.ToolCall{Args: map[string]string{"path": root, "recursive": "true"}})

	if !strings.Contains(out, "app.go") {
		t.Errorf("expected app.go in tree, got:\n%s", out)
	}
	if strings.Contains(out, ".git") {
		t.Errorf(".git should not appear in recursive tree:\n%s", out)
	}
}
