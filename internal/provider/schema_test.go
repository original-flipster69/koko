package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStringParam(t *testing.T) {
	p := StringParam("a file path")
	if p.Type != "string" {
		t.Errorf("Type: got %q, want %q", p.Type, "string")
	}
	if p.Description != "a file path" {
		t.Errorf("Description: got %q, want %q", p.Description, "a file path")
	}
}

func TestIntParam(t *testing.T) {
	p := IntParam("line number")
	if p.Type != "integer" {
		t.Errorf("Type: got %q, want %q", p.Type, "integer")
	}
	if p.Description != "line number" {
		t.Errorf("Description: got %q, want %q", p.Description, "line number")
	}
}

func TestBoolParam(t *testing.T) {
	p := BoolParam("force flag")
	if p.Type != "boolean" {
		t.Errorf("Type: got %q, want %q", p.Type, "boolean")
	}
}

func TestSchema_MarshalsToValidJSONSchema(t *testing.T) {
	s := Schema{
		Type: "object",
		Properties: map[string]Property{
			"path": StringParam("file path"),
		},
		Required: []string{"path"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		`"type":"object"`,
		`"properties":{`,
		`"path":{"type":"string","description":"file path"}`,
		`"required":["path"]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %s", want, got)
		}
	}
}

func TestSchema_EmptyPropertiesAreOmitted(t *testing.T) {
	s := Schema{Type: "object"}
	data, _ := json.Marshal(s)
	got := string(data)
	if strings.Contains(got, "properties") {
		t.Errorf("expected properties to be omitted for empty schema, got: %s", got)
	}
	if strings.Contains(got, "required") {
		t.Errorf("expected required to be omitted when empty, got: %s", got)
	}
}

func TestSchema_DescriptionOmittedWhenEmpty(t *testing.T) {
	p := Property{Type: "string"}
	data, _ := json.Marshal(p)
	got := string(data)
	if strings.Contains(got, "description") {
		t.Errorf("expected description to be omitted when empty, got: %s", got)
	}
}

func TestToolDef_FullJSONRoundTrip(t *testing.T) {
	td := ToolDef{
		Name:        "read_file",
		Description: "reads a file",
		Params: Schema{
			Type: "object",
			Properties: map[string]Property{
				"path":   StringParam("file path"),
				"offset": StringParam("start line"),
			},
			Required: []string{"path"},
		},
	}
	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back ToolDef
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.Name != td.Name {
		t.Errorf("Name lost: %q → %q", td.Name, back.Name)
	}
	if back.Params.Type != td.Params.Type {
		t.Errorf("schema Type lost: %q → %q", td.Params.Type, back.Params.Type)
	}
	if len(back.Params.Required) != 1 || back.Params.Required[0] != "path" {
		t.Errorf("Required lost: %v", back.Params.Required)
	}
	if back.Params.Properties["path"].Description != "file path" {
		t.Errorf("property description lost: %q", back.Params.Properties["path"].Description)
	}
}
