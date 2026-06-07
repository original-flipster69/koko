package cage

import (
	"strings"
	"testing"
)

func TestGenerateDarwin(t *testing.T) {
	s, err := Generate("agent", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if s.Filename != "cage-agent.sh" {
		t.Errorf("filename = %q", s.Filename)
	}
	for _, want := range []string{"dscl", "createhomedir", `NEW_USER="agent"`, "collabo", "chmod -R 2770"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("darwin script missing %q", want)
		}
	}
	if strings.Contains(s.Body, "useradd") {
		t.Error("darwin script should not contain linux useradd")
	}
}

func TestGenerateLinux(t *testing.T) {
	s, err := Generate("agent", "linux")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"useradd", "groupadd", "chpasswd", "collabo", "chmod -R 2770"} {
		if !strings.Contains(s.Body, want) {
			t.Errorf("linux script missing %q", want)
		}
	}
	if strings.Contains(s.Body, "dscl") {
		t.Error("linux script should not contain darwin dscl")
	}
}

func TestGenerateUnsupportedOS(t *testing.T) {
	if _, err := Generate("agent", "windows"); err == nil {
		t.Error("expected error for unsupported OS")
	}
}

func TestGenerateRejectsBadUsernames(t *testing.T) {
	bad := []string{"", "1abc", "Has Space", "UPPER", "semi;colon", "a/b", strings.Repeat("x", 40)}
	for _, u := range bad {
		if _, err := Generate(u, "linux"); err == nil {
			t.Errorf("expected rejection of username %q", u)
		}
	}
}

func TestGeneratePasswordIsRandomAndShellSafe(t *testing.T) {
	s1, _ := Generate("agent", "linux")
	s2, _ := Generate("agent", "linux")
	if s1.Body == s2.Body {
		t.Error("two generations produced identical scripts; password not random")
	}
	for _, bad := range []string{`PASSWORD="$`, "PASSWORD=\"`", `PASSWORD="\`, `PASSWORD="""`} {
		if strings.Contains(s1.Body, bad) {
			t.Errorf("password contains shell-unsafe sequence near %q", bad)
		}
	}
}
