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

func labelSet(d configDiff) (live, restart string) {
	return strings.Join(d.liveLabels(), ", "), strings.Join(d.restartLabels(), ", ")
}

func TestDiffNoChanges(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	d := diffConfig(cur, next)
	if len(d.liveLabels()) != 0 || len(d.restartLabels()) != 0 {
		live, restart := labelSet(d)
		t.Errorf("expected no changes, got live=[%s] restart=[%s]", live, restart)
	}
}

func TestDiffLiveFields(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Model = "claude-opus-4-8"
	next.Llm.MaxSessionTokens = 2000
	next.Style.ThinkingVerbs = []string{"pondering", "scheming"}
	next.Sandbox.ScrubPII = false
	next.Sandbox.Exec.Profile = config.Strict

	d := diffConfig(cur, next)
	live, restart := labelSet(d)
	if restart != "" {
		t.Errorf("unexpected restart items: %s", restart)
	}
	for _, want := range []string{"model", "max session tokens", "thinking verbs", "scrub_pii", "exec profile"} {
		if !strings.Contains(live, want) {
			t.Errorf("live %q missing %q", live, want)
		}
	}
}

func TestDiffProviderRebuildHidesModel(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Llm.Provider = config.Mistral
	next.Llm.Model = "mistral-large-latest"

	d := diffConfig(cur, next)
	if !d.provider {
		t.Error("provider change should be a live provider rebuild")
	}
	if d.model {
		t.Error("model must not be a separate live field when the provider changed")
	}
	live, restart := labelSet(d)
	if !strings.Contains(live, "provider") {
		t.Errorf("live %q missing provider", live)
	}
	if restart != "" {
		t.Errorf("provider swap should not require restart, got %s", restart)
	}
}

func TestDiffColorSchemeIsLive(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Style.ColorScheme = map[string]int{"primary": 42}

	d := diffConfig(cur, next)
	if !d.colorScheme {
		t.Error("color scheme change should be live")
	}
	if live, _ := labelSet(d); !strings.Contains(live, "color scheme") {
		t.Errorf("live %q missing color scheme", live)
	}
}

func TestDiffSandboxNeedsRestart(t *testing.T) {
	cur := baseConfig()
	next := baseConfig()
	next.Sandbox.Root = "/other"
	next.Sandbox.AdditionalDirs = []string{"/extra"}
	next.Sandbox.DenyFiles = []string{"*.foo"}
	next.Sandbox.MaxFileSize = 4242

	d := diffConfig(cur, next)
	if len(d.liveLabels()) != 0 {
		t.Errorf("sandbox boundary changes must not be live-applied, got %v", d.liveLabels())
	}
	_, restart := labelSet(d)
	for _, want := range []string{"sandbox root", "additional dirs", "deny files", "max file size"} {
		if !strings.Contains(restart, want) {
			t.Errorf("restart %q missing %q", restart, want)
		}
	}
}
