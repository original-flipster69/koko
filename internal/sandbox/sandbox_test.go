package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathDeniesProtectedDirs(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"main.go", ".git/config", ".git/objects/ab/cd", "sub/.git/HEAD", ".koko/pipeline.toml", "sub/.koko/config.toml"} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sb, err := New(root, []string{root}, nil, 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := sb.ValidatePath(filepath.Join(root, "main.go")); err != nil {
		t.Errorf("main.go should be allowed: %v", err)
	}
	for _, denied := range []string{".git/config", ".git/objects/ab/cd", "sub/.git/HEAD", ".koko/pipeline.toml", "sub/.koko/config.toml"} {
		if _, err := sb.ValidatePath(filepath.Join(root, denied)); err == nil {
			t.Errorf("%q must be denied (inside protected dir)", denied)
		}
	}
}
