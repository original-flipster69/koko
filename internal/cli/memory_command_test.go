package cli

import (
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/memories"
	"github.com/original-flipster69/koko/internal/ui"
)

func memCmd(store *memories.Store, line string) string {
	return memoryCommand(store, ui.DefaultScheme(), line, strings.Fields(line))
}

func TestMemoryCommandCRUD(t *testing.T) {
	store, err := memories.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if out := memCmd(store, ":memories"); !strings.Contains(out, "none stored") {
		t.Errorf("empty list: got %q", out)
	}

	out := memCmd(store, ":memories add likes-go prefers tabs over spaces")
	if !strings.Contains(out, "saved") {
		t.Fatalf("add: got %q", out)
	}

	mem, ok, _ := store.Get("likes-go")
	if !ok {
		t.Fatal("memories not persisted")
	}
	if mem.Body != "prefers tabs over spaces" {
		t.Errorf("body: got %q", mem.Body)
	}
	if mem.Type != memories.TypeProject {
		t.Errorf("type: got %q, want project", mem.Type)
	}

	if out := memCmd(store, ":memories"); !strings.Contains(out, "likes-go") {
		t.Errorf("list after add: got %q", out)
	}

	if out := memCmd(store, ":memories likes-go"); !strings.Contains(out, "prefers tabs over spaces") {
		t.Errorf("read: got %q", out)
	}

	if out := memCmd(store, ":memories delete likes-go"); !strings.Contains(out, "deleted") {
		t.Errorf("delete: got %q", out)
	}
	if _, ok, _ := store.Get("likes-go"); ok {
		t.Error("memories still present after delete")
	}
}

func TestMemoryCommandErrors(t *testing.T) {
	store, _ := memories.Open(t.TempDir())
	cases := []struct {
		line, want string
	}{
		{":memories missing", "no memories named"},
		{":memories add onlyname", "usage"},
		{":memories delete ghost", "no memories named"},
	}
	for _, c := range cases {
		if out := memCmd(store, c.line); !strings.Contains(strings.ToLower(out), c.want) {
			t.Errorf("%q: got %q, want substring %q", c.line, out, c.want)
		}
	}
}
