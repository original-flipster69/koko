package cage

import (
	"strings"
	"testing"
)

func TestGenerateDarwin(t *testing.T) {
	s, err := Generate(Options{Username: "agent", GOOS: "darwin"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Filename != "cage-agent.sh" {
		t.Errorf("filename = %q", s.Filename)
	}
	for _, want := range []string{"dscl", "createhomedir", `NEW_USER="agent"`, `GROUP="collabo"`, "chmod -R 2770"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("darwin script missing %q", want)
		}
	}
	if strings.Contains(s.Body, "useradd") {
		t.Error("darwin script should not contain linux useradd")
	}
}

func TestGenerateLinux(t *testing.T) {
	s, err := Generate(Options{Username: "agent", GOOS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"useradd", "groupadd", "chpasswd", `GROUP="collabo"`, "chmod -R 2770"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("linux script missing %q", want)
		}
	}
	if strings.Contains(s.Body, "dscl") {
		t.Error("linux script should not contain darwin dscl")
	}
}

func TestGenerateDefaultGroup(t *testing.T) {
	s, err := Generate(Options{Username: "agent", GOOS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Group != DefaultGroup {
		t.Errorf("group = %q, want default %q", s.Group, DefaultGroup)
	}
}

func TestGenerateCustomGroup(t *testing.T) {
	s, err := Generate(Options{Username: "agent", Group: "devs", GOOS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Group != "devs" {
		t.Errorf("group = %q, want devs", s.Group)
	}
	if !strings.Contains(s.Body, `GROUP="devs"`) {
		t.Error("script should use the custom group")
	}
	if strings.Contains(s.Body, "collabo") {
		t.Error("script should not reference the default group when overridden")
	}
}

func TestGenerateOSOverride(t *testing.T) {
	s, err := Generate(Options{Username: "agent", GOOS: "darwin"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Body, "dscl") {
		t.Error("os override to darwin should produce a dscl script regardless of host")
	}
}

func TestGenerateUnsupportedOS(t *testing.T) {
	if _, err := Generate(Options{Username: "agent", GOOS: "windows"}); err == nil {
		t.Error("expected error for unsupported OS")
	}
}

func TestGenerateRejectsBadUsernames(t *testing.T) {
	bad := []string{"", "1abc", "Has Space", "UPPER", "semi;colon", "a/b", strings.Repeat("x", 40)}
	for _, u := range bad {
		if _, err := Generate(Options{Username: u, GOOS: "linux"}); err == nil {
			t.Errorf("expected rejection of username %q", u)
		}
	}
}

func TestGenerateRejectsBadGroups(t *testing.T) {
	bad := []string{"1bad", "Has Space", "semi;colon", "a/b"}
	for _, g := range bad {
		if _, err := Generate(Options{Username: "agent", Group: g, GOOS: "linux"}); err == nil {
			t.Errorf("expected rejection of group %q", g)
		}
	}
}

func TestGeneratePasswordIsRandomAndShellSafe(t *testing.T) {
	s1, _ := Generate(Options{Username: "agent", GOOS: "linux"})
	s2, _ := Generate(Options{Username: "agent", GOOS: "linux"})
	if s1.Body == s2.Body {
		t.Error("two generations produced identical scripts; password not random")
	}
	for _, bad := range []string{`PASSWORD="$`, "PASSWORD=\"`", `PASSWORD="\`, `PASSWORD="""`} {
		if strings.Contains(s1.Body, bad) {
			t.Errorf("password contains shell-unsafe sequence near %q", bad)
		}
	}
}
