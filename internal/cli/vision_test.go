package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/ignore"
	"github.com/original-flipster69/koko/internal/sandbox"
)

func TestVisibleFiles(t *testing.T) {
	root := t.TempDir()
	mkfile := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkfile("main.go")
	mkfile("src/app.go")
	mkfile(".env")             // denied
	mkfile("secret.key")       // denied (*.key)
	mkfile("build/output.bin") // ignored dir
	mkfile("notes.log")        // ignored pattern
	mkfile(".git/config")      // skipped noise dir
	mkfile("node_modules/lib/index.js")

	deny := []string{".env", ".env.*", "*.key"}
	sb, err := sandbox.New(root, []string{root}, deny, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	ig := ignore.NewFromPatterns([]string{"build/", "*.log"})

	files, capped, err := visibleFiles(sb, ig)
	if err != nil {
		t.Fatal(err)
	}
	if capped {
		t.Error("unexpected cap")
	}
	got := strings.Join(files, ",")
	want := "main.go,node_modules/lib/index.js,src/app.go"
	if got != want {
		t.Errorf("visible files = %q, want %q", got, want)
	}
	for _, hidden := range []string{".env", "secret.key", "build/output.bin", "notes.log", ".git/config"} {
		for _, f := range files {
			if f == hidden {
				t.Errorf("%q should not be visible", hidden)
			}
		}
	}
}
