package pushpuppet

import (
	"testing"

	"github.com/original-flipster69/koko/internal/provider"
)

func TestToolCallSig(t *testing.T) {
	a := provider.ToolCall{Name: "search_files", Args: map[string]string{"pattern": "x", "glob": "*.go"}}
	b := provider.ToolCall{Name: "search_files", Args: map[string]string{"glob": "*.go", "pattern": "x"}}
	if toolCallSig(a) != toolCallSig(b) {
		t.Error("identical calls with reordered args must produce the same signature")
	}

	c := provider.ToolCall{Name: "search_files", Args: map[string]string{"pattern": "y", "glob": "*.go"}}
	if toolCallSig(a) == toolCallSig(c) {
		t.Error("calls with different arg values must produce different signatures")
	}

	d := provider.ToolCall{Name: "list_dir", Args: map[string]string{"pattern": "x", "glob": "*.go"}}
	if toolCallSig(a) == toolCallSig(d) {
		t.Error("calls to different tools must produce different signatures")
	}
}

func TestToolRegistry_AllEntriesWellFormed(t *testing.T) {
	for _, tc := range tools {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Name == "" {
				t.Error("Name is empty")
			}
			if tc.Description == "" {
				t.Error("Description is empty")
			}
			if tc.Handler == nil {
				t.Error("Handler is nil")
			}
			if tc.Params.Type == "" {
				t.Error("Params.Type is empty")
			}
			for _, req := range tc.Params.Required {
				if _, ok := tc.Params.Properties[req]; !ok {
					t.Errorf("Required arg %q has no Property entry", req)
				}
			}
			for name, prop := range tc.Params.Properties {
				if prop.Type == "" {
					t.Errorf("Property %q has empty Type", name)
				}
				if prop.Description == "" {
					t.Errorf("Property %q has empty Description", name)
				}
			}
		})
	}
}

func TestToolRegistry_NamesUnique(t *testing.T) {
	seen := make(map[string]bool, len(tools))
	for _, tc := range tools {
		if seen[tc.Name] {
			t.Errorf("duplicate tool name: %q", tc.Name)
		}
		seen[tc.Name] = true
	}
}

func TestToolsByName_IndexMatchesSlice(t *testing.T) {
	if len(toolsByName) != len(tools) {
		t.Fatalf("toolsByName has %d entries, tools has %d", len(toolsByName), len(tools))
	}
	for i := range tools {
		got, ok := toolsByName[tools[i].Name]
		if !ok {
			t.Errorf("missing index entry for %q", tools[i].Name)
			continue
		}
		if got != &tools[i] {
			t.Errorf("index entry for %q does not point at slice element", tools[i].Name)
		}
	}
}

func TestToolVerb_KnownToolReturnsConfiguredVerb(t *testing.T) {
	if got := toolVerb("read_file"); got != "◇ reading" {
		t.Errorf("read_file verb: got %q, want %q", got, "◇ reading")
	}
}

func TestToolVerb_UnknownToolReturnsFallback(t *testing.T) {
	if got := toolVerb("does_not_exist"); got != "working" {
		t.Errorf("unknown tool verb: got %q, want %q", got, "working")
	}
}

func TestToolReadOnly_ReadFileIsReadOnly(t *testing.T) {
	if !toolReadOnly("read_file") {
		t.Error("read_file should be ReadOnly")
	}
	if toolReadOnly("write_file") {
		t.Error("write_file should NOT be ReadOnly")
	}
	if toolReadOnly("does_not_exist") {
		t.Error("unknown tool should default to not-ReadOnly")
	}
}

func TestToolQuiet_ReadFileIsQuiet(t *testing.T) {
	if !toolQuiet("read_file") {
		t.Error("read_file should be Quiet")
	}
	if toolQuiet("write_file") {
		t.Error("write_file should NOT be Quiet")
	}
}

func TestBuildTools_ReturnsAllToolsInOrder(t *testing.T) {
	a := &PushPuppet{}
	defs := a.buildTools()
	if len(defs) != len(tools) {
		t.Fatalf("buildTools returned %d, expected %d", len(defs), len(tools))
	}
	for i, td := range defs {
		if td.Name != tools[i].Name {
			t.Errorf("order mismatch at %d: got %q, want %q", i, td.Name, tools[i].Name)
		}
		if td.Description != tools[i].Description {
			t.Errorf("Description at %d wrong", i)
		}
		if td.Params.Type != tools[i].Params.Type {
			t.Errorf("Params.Type at %d wrong", i)
		}
	}
}
