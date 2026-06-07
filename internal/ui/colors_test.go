package ui

import "testing"

func TestApplyColorsOverridesPalette(t *testing.T) {
	origErr, origPrimary := Red, Blueberry
	defer func() { Red, Blueberry = origErr, origPrimary }()

	if err := ApplyColors(map[string]int{"error": 1, "primary": 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Red != fg(1) {
		t.Errorf("error role not applied: got %q, want %q", Red, fg(1))
	}
	if Blueberry != fg(2) {
		t.Errorf("primary role not applied: got %q, want %q", Blueberry, fg(2))
	}
}

func TestApplyColorsNilIsNoop(t *testing.T) {
	orig := Blueberry
	if err := ApplyColors(nil); err != nil {
		t.Fatalf("nil overrides should be a no-op, got %v", err)
	}
	if Blueberry != orig {
		t.Errorf("palette changed on nil input")
	}
}

func TestApplyColorsRejectsUnknownKey(t *testing.T) {
	if err := ApplyColors(map[string]int{"chartreuse": 42}); err == nil {
		t.Error("expected error for unknown color key")
	}
}

func TestApplyColorsRejectsOutOfRange(t *testing.T) {
	for _, code := range []int{-1, 256} {
		if err := ApplyColors(map[string]int{"error": code}); err == nil {
			t.Errorf("expected out-of-range error for code %d", code)
		}
	}
}

func TestApplyColorsBackgroundAndSplash(t *testing.T) {
	origBg, origSplash := diffBgGreen, splashColorCode
	defer func() {
		diffBgGreen, splashColorCode = origBg, origSplash
		rebuildSplashStyles()
	}()

	if err := ApplyColors(map[string]int{"diff_add_bg": 28, "splash": 200}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diffBgGreen != bg(28) {
		t.Errorf("diff_add_bg not overridden: got %q", diffBgGreen)
	}
	if splashColorCode != 200 {
		t.Errorf("splash code not set: got %d", splashColorCode)
	}
}
