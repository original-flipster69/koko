package cli

import (
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/config"
)

func baseConfig() *config.Config {
	return &config.Config{
		Llm: config.LlmConfig{
			Provider:         config.Anthropic,
			Model:            "claude-sonnet-4-6",
			MaxSessionTokens: 1000,
		},
		Sandbox: config.SandboxConfig{Root: "/work", ScrubPII: true},
		Style:   config.StyleConfig{ThinkingVerbs: []string{"thinking"}},
	}
}

type capture struct {
	model    string
	verbs    []string
	maxTok   int
	modelHit bool
}

func (c *capture) setModel(m string)   { c.model = m; c.modelHit = true }
func (c *capture) setVerbs(v []string) { c.verbs = v }
func (c *capture) setMaxTokens(n int)  { c.maxTok = n }

func TestReloadNoChanges(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	c := &capture{}
	applied, restart := applyReloadedConfig(cur, next, c.setModel, c.setVerbs, c.setMaxTokens)
	if len(applied) != 0 || len(restart) != 0 {
		t.Errorf("expected no changes, got applied=%v restart=%v", applied, restart)
	}
	if c.modelHit {
		t.Error("setModel should not be called when model is unchanged")
	}
}

func TestReloadAppliesLiveChanges(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Model = "claude-opus-4-8"
	next.Llm.MaxSessionTokens = 2000
	next.Style.ThinkingVerbs = []string{"pondering", "scheming"}

	c := &capture{}
	applied, restart := applyReloadedConfig(cur, next, c.setModel, c.setVerbs, c.setMaxTokens)

	if len(restart) != 0 {
		t.Errorf("unexpected restart items: %v", restart)
	}
	want := "model, max session tokens, thinking verbs"
	got := strings.Join(applied, ", ")
	for _, item := range []string{"model", "max session tokens", "thinking verbs"} {
		if !strings.Contains(got, item) {
			t.Errorf("applied %q missing %q (want all of: %s)", got, item, want)
		}
	}
	if c.model != "claude-opus-4-8" {
		t.Errorf("setModel got %q", c.model)
	}
	if c.maxTok != 2000 {
		t.Errorf("setMaxTokens got %d", c.maxTok)
	}
	if cur.Llm.Model != "claude-opus-4-8" {
		t.Error("live config Model not updated for :config consistency")
	}
}

func TestReloadProviderChangeNeedsRestart(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Provider = config.Mistral
	next.Llm.Model = "mistral-large-latest"

	c := &capture{}
	applied, restart := applyReloadedConfig(cur, next, c.setModel, c.setVerbs, c.setMaxTokens)

	if c.modelHit {
		t.Error("model must not be swapped when the provider changed (needs restart)")
	}
	if len(applied) != 0 {
		t.Errorf("nothing should be live-applied, got %v", applied)
	}
	if strings.Join(restart, ",") != "provider" {
		t.Errorf("restart should be [provider], got %v", restart)
	}
}

func TestReloadRestartRequiredFields(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Sandbox.Root = "/other"
	next.Sandbox.ScrubPII = false
	next.Sandbox.Exec.Profile = config.Strict

	c := &capture{}
	_, restart := applyReloadedConfig(cur, next, c.setModel, c.setVerbs, c.setMaxTokens)
	for _, item := range []string{"sandbox root", "scrub_pii", "exec profile"} {
		found := false
		for _, r := range restart {
			if r == item {
				found = true
			}
		}
		if !found {
			t.Errorf("restart list missing %q: %v", item, restart)
		}
	}
}
