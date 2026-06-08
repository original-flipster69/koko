package ui

import "testing"

func TestApplyColorsOverridesRoles(t *testing.T) {
	origErr, origPrimary := Danger, Primary
	defer func() { Danger, Primary = origErr, origPrimary }()

	if err := ApplyColors(map[string]int{"error": 1, "primary": 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Danger != fg(1) {
		t.Errorf("error role not applied: got %q, want %q", Danger, fg(1))
	}
	if Primary != fg(2) {
		t.Errorf("primary role not applied: got %q, want %q", Primary, fg(2))
	}
}

func TestApplyColorsLeavesPaletteUntouched(t *testing.T) {
	origPrimary := Primary
	defer func() { Primary = origPrimary }()

	if err := ApplyColors(map[string]int{"primary": 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Blueberry != "\033[38;5;99m" {
		t.Errorf("pigment constant Blueberry was mutated: %q", Blueberry)
	}
	if Primary == Blueberry {
		t.Error("primary role should now differ from its default pigment")
	}
}

func TestApplyColorsNilIsNoop(t *testing.T) {
	orig := Primary
	if err := ApplyColors(nil); err != nil {
		t.Fatalf("nil overrides should be a no-op, got %v", err)
	}
	if Primary != orig {
		t.Errorf("roles changed on nil input")
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
	origBg, origSplash := DiffAddBg, splashColorCode
	defer func() {
		DiffAddBg, splashColorCode = origBg, origSplash
		rebuildSplashStyles()
	}()

	if err := ApplyColors(map[string]int{"diff_add_bg": 28, "splash": 200}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if DiffAddBg != bg(28) {
		t.Errorf("diff_add_bg not overridden: got %q", DiffAddBg)
	}
	if splashColorCode != 200 {
		t.Errorf("splash code not set: got %d", splashColorCode)
	}
}
