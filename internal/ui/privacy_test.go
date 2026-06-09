package ui

import (
	"strings"
	"testing"
)

func TestPrivacyWarningLocalProviderIsSilent(t *testing.T) {
	if got := PrivacyWarning("ollama"); got != "" {
		t.Errorf("ollama should produce no privacy warning, got %q", got)
	}
}

func TestPrivacyWarningRemoteProviders(t *testing.T) {
	for _, p := range []string{"claude", "mistral"} {
		got := PrivacyWarning(p)
		if got == "" {
			t.Errorf("provider %q should produce a privacy warning", p)
		}
		if !strings.Contains(got, p) {
			t.Errorf("warning for %q should name the provider, got %q", p, got)
		}
		if !strings.Contains(strings.ToLower(got), "privacy") {
			t.Errorf("warning for %q should mention privacy, got %q", p, got)
		}
	}
}
