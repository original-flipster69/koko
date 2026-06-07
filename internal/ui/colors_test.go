package ui

import "testing"

func TestApplyColorsOverridesPalette(t *testing.T) {
	orig := Red
	defer func() { Red = orig }()

	if err := ApplyColors(map[string]int{"red": 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Red != fg(1) {
		t.Errorf("red not overridden: got %q, want %q", Red, fg(1))
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
		if err := ApplyColors(map[string]int{"red": code}); err == nil {
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
