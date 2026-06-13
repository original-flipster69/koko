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

func TestMistralPromptCacheKey(t *testing.T) {
	m, err := newMistral("key", "mistral-large-latest", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.cacheKey == "" {
		t.Fatal("expected a non-empty prompt cache key")
	}
	req := mistralReq{Model: m.model, PromptCacheKey: m.cacheKey}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"prompt_cache_key":"`+m.cacheKey+`"`) {
		t.Errorf("serialized request missing prompt_cache_key: %s", data)
	}
	m2, _ := newMistral("key", "mistral-large-latest", "")
	if m.cacheKey == m2.cacheKey {
		t.Error("cache keys should be unique per session")
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
