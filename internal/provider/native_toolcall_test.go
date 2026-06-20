package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToMistralMsgs_NativeToolRoundTrip(t *testing.T) {
	msgs := []Msg{
		{Role: System, Content: "sys"},
		{Role: User, Content: "read it"},
		{Role: Assistant, ToolCalls: []ToolCall{{ID: "abc123def", Name: "read_file", Args: map[string]string{"path": "go.mod"}}}},
		{Role: Tool, ToolCallID: "abc123def", ToolName: "read_file", Content: "module x"},
	}
	out := toMistralMsgs(msgs)
	if len(out) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(out))
	}

	asst := out[2]
	if asst.Role != "assistant" || len(asst.ToolCalls) != 1 {
		t.Fatalf("assistant tool_calls missing: %+v", asst)
	}
	if asst.ToolCalls[0].Id != "abc123def" || asst.ToolCalls[0].Type != "function" {
		t.Errorf("tool call id/type wrong: %+v", asst.ToolCalls[0])
	}
	if asst.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("tool call name wrong: %q", asst.ToolCalls[0].Function.Name)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(asst.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("arguments not valid JSON: %v", err)
	}
	if args["path"] != "go.mod" {
		t.Errorf("arguments wrong: %v", args)
	}

	tool := out[3]
	if tool.Role != "tool" || tool.ToolCallID != "abc123def" || tool.Name != "read_file" {
		t.Errorf("tool message wrong: %+v", tool)
	}
	if tool.Content != "module x" {
		t.Errorf("tool content wrong: %q", tool.Content)
	}
}

func TestEnsureToolID(t *testing.T) {
	if got := ensureToolID("abc123def"); got != "abc123def" {
		t.Errorf("valid id should pass through, got %q", got)
	}
	for _, bad := range []string{"", "short", "has-dash01", "waytoolongforanid"} {
		got := ensureToolID(bad)
		if !validToolID(got) {
			t.Errorf("ensureToolID(%q) = %q, not a valid 9-char alnum id", bad, got)
		}
	}
}

func TestToMistralMsgs_OrphanToolBecomesUser(t *testing.T) {
	// A tool message not preceded by an assistant tool_calls (e.g. a poisoned
	// session) must not be sent with role "tool" — Mistral rejects that ordering.
	msgs := []Msg{
		{Role: User, Content: "hi"},
		{Role: Tool, ToolCallID: "x", ToolName: "read_file", Content: "orphan"},
	}
	out := toMistralMsgs(msgs)
	for i, m := range out {
		if m.Role == "tool" {
			t.Errorf("message %d emitted as role tool after non-assistant: %+v", i, m)
		}
	}
	// the orphan content should survive somewhere
	joined := ""
	for _, m := range out {
		if s, ok := m.Content.(string); ok {
			joined += s
		}
	}
	if !strings.Contains(joined, "orphan") {
		t.Errorf("orphan tool content lost: %q", joined)
	}
}

func TestToMistralMsgs_ValidToolStaysTool(t *testing.T) {
	msgs := []Msg{
		{Role: Assistant, ToolCalls: []ToolCall{{ID: "id1", Name: "read_file", Args: map[string]string{"path": "a"}}}},
		{Role: Tool, ToolCallID: "id1", ToolName: "read_file", Content: "ok"},
	}
	out := toMistralMsgs(msgs)
	if len(out) != 2 || out[1].Role != "tool" {
		t.Fatalf("valid tool message should stay role tool: %+v", out)
	}
}

func TestFlattenToolMessages_LegacyShape(t *testing.T) {
	msgs := []Msg{
		{Role: User, Content: "do it"},
		{Role: Assistant, ToolCalls: []ToolCall{{ID: "id1", Name: "read_file", Args: map[string]string{"path": "a"}}}},
		{Role: Tool, ToolCallID: "id1", ToolName: "read_file", Content: "alpha"},
		{Role: Tool, ToolCallID: "id2", ToolName: "read_file", Content: "beta"},
	}
	out := flattenToolMessages(msgs)
	if len(out) != 3 {
		t.Fatalf("expected 3 flattened messages, got %d: %+v", len(out), out)
	}
	if out[1].Role != Assistant || out[1].Content != "[calling tools]" {
		t.Errorf("assistant turn not flattened to placeholder: %+v", out[1])
	}
	if out[2].Role != User {
		t.Fatalf("tool results should coalesce into one user msg, got role %q", out[2].Role)
	}
	for _, want := range []string{toolResultsPreamble, `<tool_output name="read_file">`, "alpha", "beta"} {
		if !strings.Contains(out[2].Content, want) {
			t.Errorf("flattened tool results missing %q:\n%s", want, out[2].Content)
		}
	}
}
