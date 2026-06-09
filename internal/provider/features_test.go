package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeCacheBreakpoints(t *testing.T) {
	a, err := newClaude("key", "claude-sonnet-4-6", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := []Msg{
		{Role: System, Content: "system prompt"},
		{Role: User, Content: "hello"},
	}
	tools := []ToolDef{{Name: "a"}, {Name: "b"}}
	req := a.request(msgs, tools, false)

	if req.Tools[len(req.Tools)-1].CacheControl == nil {
		t.Error("last tool missing cache_control")
	}
	if req.Tools[0].CacheControl != nil {
		t.Error("non-last tool should not have cache_control")
	}
	sysBlocks, ok := req.System.([]map[string]interface{})
	if !ok {
		t.Fatalf("system should be cache block array, got %T", req.System)
	}
	if sysBlocks[0]["cache_control"] == nil {
		t.Error("system block missing cache_control")
	}
	lastBlocks, ok := req.Msgs[len(req.Msgs)-1].Content.([]map[string]interface{})
	if !ok {
		t.Fatalf("last message should be cache block array, got %T", req.Msgs[len(req.Msgs)-1].Content)
	}
	if lastBlocks[len(lastBlocks)-1]["cache_control"] == nil {
		t.Error("last message block missing cache_control")
	}
}

func TestHasMsgPrefix(t *testing.T) {
	full := []Msg{{Role: User, Content: "a"}, {Role: Assistant, Content: "b"}, {Role: User, Content: "c"}}
	if !hasMsgPrefix(full, full[:2]) {
		t.Error("expected prefix match")
	}
	if hasMsgPrefix(full[:2], full) {
		t.Error("longer prefix than full must not match")
	}
	if hasMsgPrefix(full, []Msg{{Role: User, Content: "x"}}) {
		t.Error("differing content must not match")
	}
}

func TestAssistantPlaceholder(t *testing.T) {
	if got := assistantPlaceholder(&Response{Content: "hi"}); got != "hi" {
		t.Errorf("text response: got %q", got)
	}
	got := assistantPlaceholder(&Response{ToolCalls: []ToolCall{{Name: "read_file"}, {Name: "list_dir"}}})
	if got != "[calling tools: read_file, list_dir]" {
		t.Errorf("tool placeholder: got %q", got)
	}
}

func TestConvRespToResponse(t *testing.T) {
	raw := `{
		"conversation_id": "c1",
		"outputs": [
			{"type": "message.output", "role": "assistant", "content": "hello"},
			{"type": "function.call", "name": "read_file", "arguments": "{\"path\": \"main.go\"}"}
		],
		"usage": {"prompt_tokens": 12, "completion_tokens": 7}
	}`
	var cr convResp
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatal(err)
	}
	resp := cr.toResponse()
	if resp.Content != "hello" {
		t.Errorf("content: got %q", resp.Content)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 7 {
		t.Errorf("usage: got %+v", resp.Usage)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "read_file" || resp.ToolCalls[0].Args["path"] != "main.go" {
		t.Errorf("tool calls: got %+v", resp.ToolCalls)
	}
}

func TestToConvInputs(t *testing.T) {
	in := toConvInputs([]Msg{{Role: User, Content: "hi"}})
	if len(in) != 1 || in[0].Type != "message.input" || in[0].Role != "user" || in[0].Content != "hi" {
		t.Errorf("got %+v", in)
	}
}

func TestMistralSetModelResetsConversation(t *testing.T) {
	m, err := newMistral("key", "mistral-large-latest", "", true)
	if err != nil {
		t.Fatal(err)
	}
	m.convID = "abc"
	m.committed = []Msg{{Role: User, Content: "x"}}
	m.SetModel("mistral-medium")
	if m.convID != "" || m.committed != nil {
		t.Error("switching model must reset the server-side conversation")
	}
}

func TestClaudeUsageTagsParse(t *testing.T) {
	raw := `{"input_tokens": 10, "output_tokens": 20, "cache_creation_input_tokens": 5, "cache_read_input_tokens": 100}`
	var u claudeUsg
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatal(err)
	}
	if u.Input != 10 || u.Output != 20 || u.CacheCreation != 5 || u.CacheRead != 100 {
		t.Errorf("usage parse mismatch: %+v", u)
	}
}

func TestClaudeRequestMarshals(t *testing.T) {
	a, _ := newClaude("key", "claude-sonnet-4-6", "", 0)
	req := a.request([]Msg{{Role: System, Content: "sys"}, {Role: User, Content: "hi"}}, []ToolDef{{Name: "a"}}, true)
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cache_control") {
		t.Error("serialized request should contain cache_control")
	}
}
