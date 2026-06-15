package config

import (
	"strings"
	"testing"
)

func baseConfig() *Config {
	return &Config{
		Llm: LlmConfig{
			Provider:         Anthropic,
			Model:            "claude-sonnet-4-6",
			MaxSessionTokens: 1000,
		},
		Sandbox: SandboxConfig{Root: "/work", ScrubPII: true},
		Style:   StyleConfig{ThinkingVerbs: []string{"thinking"}},
	}
}

func TestDiffNoChanges(t *testing.T) {
	d := baseConfig().Diff(baseConfig())
	if (d != Diff{}) {
		t.Errorf("expected zero diff, got %+v", d)
	}
}

func TestDiffLiveFields(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Model = "claude-opus-4-8"
	next.Llm.MaxSessionTokens = 2000
	next.Style.ThinkingVerbs = []string{"pondering", "scheming"}
	next.Sandbox.ScrubPII = false
	next.Sandbox.Exec.Profile = Strict

	d := cur.Diff(next)
	if !d.Model || !d.MaxTokens || !d.Verbs || !d.ScrubPII || !d.ExecLimits {
		t.Errorf("expected model/maxTokens/verbs/scrubPII/execLimits live, got %+v", d)
	}
	if len(d.RestartLabels()) != 0 {
		t.Errorf("unexpected restart items: %v", d.RestartLabels())
	}
}

func TestDiffProviderRebuildHidesModel(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Provider = Mistral
	next.Llm.Model = "mistral-large-latest"

	d := cur.Diff(next)
	if !d.Provider {
		t.Error("provider change should be a live provider rebuild")
	}
	if d.Model {
		t.Error("model must not be a separate live field when the provider changed")
	}
	if len(d.RestartLabels()) != 0 {
		t.Errorf("provider swap should not require restart, got %v", d.RestartLabels())
	}
}

func TestDiffColorSchemeIsLive(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Style.ColorScheme = map[string]int{"primary": 42}

	if d := cur.Diff(next); !d.ColorScheme {
		t.Errorf("color scheme change should be live, got %+v", d)
	}
}

func TestDiffSandboxNeedsRestart(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Sandbox.Root = "/other"
	next.Sandbox.AdditionalDirs = []string{"/extra"}
	next.Sandbox.DenyFiles = []string{"*.foo"}
	next.Sandbox.MaxFileSize = 4242

	restart := strings.Join(cur.Diff(next).RestartLabels(), ", ")
	for _, want := range []string{"sandbox root", "additional dirs", "deny files", "max file size"} {
		if !strings.Contains(restart, want) {
			t.Errorf("restart %q missing %q", restart, want)
		}
	}
}
