package plays

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	raw := "---\n" +
		"description: A review play\n" +
		"provider: Claude\n" +
		"model: claude-sonnet-4-20250514\n" +
		"---\n" +
		"Review the code.\n"
	p := parse(raw)
	if p.Description != "A review play" {
		t.Errorf("description = %q", p.Description)
	}
	if p.Provider != "Claude" {
		t.Errorf("provider = %q", p.Provider)
	}
	if p.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q", p.Model)
	}
	if p.Body != "Review the code." {
		t.Errorf("body = %q", p.Body)
	}
}

func TestNameComesFromFilename(t *testing.T) {
	dir := t.TempDir()
	contents := "---\nname: ignored\ndescription: d\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("ignored"); ok {
		t.Error("play registered under frontmatter name; should use filename")
	}
	p, ok := r.Get("review")
	if !ok {
		t.Fatal("play not registered under filename")
	}
	if p.Name != "review" {
		t.Errorf("name = %q, want review", p.Name)
	}
}

func TestRenderPlaceholder(t *testing.T) {
	p := Play{Body: "Refactor {{args}} carefully."}
	if got := p.Render("auth.go"); got != "Refactor auth.go carefully." {
		t.Errorf("render = %q", got)
	}
}

func TestRenderPlaceholderEmptyArgs(t *testing.T) {
	p := Play{Body: "Refactor {{args}} carefully."}
	if got := p.Render(""); got != "Refactor  carefully." {
		t.Errorf("render = %q", got)
	}
}

func TestRenderAppendFallback(t *testing.T) {
	p := Play{Body: "Review the code."}
	want := "Review the code.\n\nUser request:\nfocus on auth"
	if got := p.Render("focus on auth"); got != want {
		t.Errorf("render = %q, want %q", got, want)
	}
}

func TestRenderNoArgsNoPlaceholder(t *testing.T) {
	p := Play{Body: "Review the code."}
	if got := p.Render(""); got != "Review the code." {
		t.Errorf("render = %q", got)
	}
}
