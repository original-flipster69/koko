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

func TestParseEffort(t *testing.T) {
	cases := map[string]struct {
		want Effort
		ok   bool
	}{
		"default": {EffortDefault, true},
		"low":     {EffortLow, true},
		"medium":  {EffortMedium, true},
		"high":    {EffortHigh, true},
		"HIGH":    {EffortDefault, false},
		"":        {EffortDefault, false},
		"bogus":   {EffortDefault, false},
	}
	for in, c := range cases {
		got, ok := ParseEffort(in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseEffort(%q) = (%v, %v), want (%v, %v)", in, got, ok, c.want, c.ok)
		}
	}
}

func TestClaudeEffortAdaptive(t *testing.T) {
	a, _ := newClaude("key", "claude-opus-4-8", "", 0)
	req := a.request([]Msg{{Role: User, Content: "hi"}}, nil, false)
	if req.Thinking != nil || req.OutputConfig != nil {
		t.Errorf("default effort must omit thinking/output_config, got thinking=%v output_config=%v", req.Thinking, req.OutputConfig)
	}
	a.SetEffort(EffortHigh)
	req = a.request([]Msg{{Role: User, Content: "hi"}}, nil, false)
	if req.Thinking == nil || req.Thinking.Type != "adaptive" {
		t.Errorf("high effort should enable adaptive thinking, got %+v", req.Thinking)
	}
	if req.OutputConfig == nil || req.OutputConfig.Effort != "high" {
		t.Errorf("high effort should set output_config.effort=high, got %+v", req.OutputConfig)
	}
	data, _ := json.Marshal(req)
	if !strings.Contains(string(data), `"output_config":{"effort":"high"}`) {
		t.Errorf("serialized request should nest effort under output_config, got %s", data)
	}
}

func TestMistralEffortMapping(t *testing.T) {
	cases := map[Effort]string{
		EffortDefault: "",
		EffortLow:     "none",
		EffortMedium:  "high",
		EffortHigh:    "high",
	}
	for level, want := range cases {
		if got := mistralReasoningEffort(level); got != want {
			t.Errorf("mistralReasoningEffort(%v) = %q, want %q", level, got, want)
		}
	}
}

func TestOllamaEffortOmittedByDefault(t *testing.T) {
	o, _ := newOllama("llama3", "")
	data, _ := json.Marshal(ollamaReq{Model: o.model, Think: string(o.effort)})
	if strings.Contains(string(data), "think") {
		t.Errorf("default effort must omit think, got %s", data)
	}
	o.SetEffort(EffortMedium)
	data, _ = json.Marshal(ollamaReq{Model: o.model, Think: string(o.effort)})
	if !strings.Contains(string(data), `"think":"medium"`) {
		t.Errorf("medium effort should serialize think=medium, got %s", data)
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
