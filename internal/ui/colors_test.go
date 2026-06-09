package ui

import "testing"

func TestWithOverridesRoles(t *testing.T) {
	s, err := DefaultScheme().With(map[string]int{"error": 1, "primary": 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Danger != fg(1) {
		t.Errorf("error role not applied: got %q, want %q", s.Danger, fg(1))
	}
	if s.Primary != fg(2) {
		t.Errorf("primary role not applied: got %q, want %q", s.Primary, fg(2))
	}
}

func TestWithDoesNotMutateReceiver(t *testing.T) {
	base := DefaultScheme()
	if _, err := base.With(map[string]int{"primary": 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base.Primary != fg(Blueberry) {
		t.Errorf("receiver scheme was mutated: %q", base.Primary)
	}
}

func TestWithNilIsDefault(t *testing.T) {
	s, err := DefaultScheme().With(nil)
	if err != nil {
		t.Fatalf("nil overrides should be a no-op, got %v", err)
	}
	if s != DefaultScheme() {
		t.Error("nil overrides should yield the default scheme")
	}
}

func TestWithRejectsUnknownKey(t *testing.T) {
	if _, err := DefaultScheme().With(map[string]int{"chartreuse": 42}); err == nil {
		t.Error("expected error for unknown color key")
	}
}

func TestWithRejectsOutOfRange(t *testing.T) {
	for _, code := range []int{-1, 256} {
		if _, err := DefaultScheme().With(map[string]int{"error": code}); err == nil {
			t.Errorf("expected out-of-range error for code %d", code)
		}
	}
}

func TestWithBackgroundAndSplash(t *testing.T) {
	s, err := DefaultScheme().With(map[string]int{"diff_add_bg": 28, "splash": 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DiffAddBg != bg(28) {
		t.Errorf("diff_add_bg not overridden: got %q", s.DiffAddBg)
	}
	if s.Splash != 200 {
		t.Errorf("splash code not set: got %d", s.Splash)
	}
}
