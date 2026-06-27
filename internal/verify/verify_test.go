package verify

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePipeline(t *testing.T) {
	data := []byte(`
[[stage]]
name = "build"
cmd = "go build ./..."
fast = true
timeout = "90s"

[[stage]]
name = "test"
cmd = "go test ./..."
`)
	p, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(p.Stages))
	}
	if !p.Stages[0].Fast || p.Stages[0].Timeout != 90*time.Second {
		t.Errorf("stage 0 fast/timeout wrong: %+v", p.Stages[0])
	}
	if p.Stages[1].Fast || p.Stages[1].Timeout != defaultStageTimeout {
		t.Errorf("stage 1 defaults wrong: %+v", p.Stages[1])
	}
	if p.Hash == "" {
		t.Error("hash must be set")
	}
}

func TestParseRejectsEmptyAndIncomplete(t *testing.T) {
	if _, err := parse([]byte(``)); err == nil {
		t.Error("empty pipeline must error")
	}
	if _, err := parse([]byte("[[stage]]\nname = \"x\"\n")); err == nil {
		t.Error("stage without cmd must error")
	}
}

func TestHashIsContentPinned(t *testing.T) {
	a, _ := parse([]byte("[[stage]]\nname=\"b\"\ncmd=\"go build\"\n"))
	b, _ := parse([]byte("[[stage]]\nname=\"b\"\ncmd=\"go build\"\n"))
	c, _ := parse([]byte("[[stage]]\nname=\"b\"\ncmd=\"rm -rf /\"\n"))
	if a.Hash != b.Hash {
		t.Error("identical content must hash equal")
	}
	if a.Hash == c.Hash {
		t.Error("different content must hash differently")
	}
}

func TestTrustTOFU(t *testing.T) {
	home := t.TempDir()
	store := NewTrustStore(home)
	proj := "/some/project"

	ok, err := store.Approved(proj, "hash1")
	if err != nil || ok {
		t.Fatalf("unapproved project must not be trusted: ok=%v err=%v", ok, err)
	}

	if err := store.Approve(proj, "hash1"); err != nil {
		t.Fatal(err)
	}
	ok, _ = store.Approved(proj, "hash1")
	if !ok {
		t.Error("approved hash must be trusted")
	}

	ok, _ = store.Approved(proj, "hash2")
	if ok {
		t.Error("changed hash must invalidate approval")
	}

	store2 := NewTrustStore(home)
	ok, _ = store2.Approved(proj, "hash1")
	if !ok {
		t.Error("approval must persist across store instances")
	}
}

func TestRunnerFastOnlyAndFailFast(t *testing.T) {
	root := t.TempDir()
	r := NewRunner(root, 0)
	p := Pipeline{Stages: []Stage{
		{Name: "fmt", Command: "true", Fast: true, Timeout: time.Minute},
		{Name: "build", Command: "false", Fast: true, Timeout: time.Minute},
		{Name: "test", Command: "true", Fast: false, Timeout: time.Minute},
	}}

	res := r.Run(context.Background(), p, true)
	if res.Passed {
		t.Error("pipeline with a failing stage must not pass")
	}
	if len(res.Stages) != 2 {
		t.Errorf("fail-fast + fastOnly should run 2 stages, got %d", len(res.Stages))
	}
	if res.Stages[1].Passed {
		t.Error("build stage should be marked failed")
	}
}

func TestRunnerStageCwdAndOutput(t *testing.T) {
	root := t.TempDir()
	r := NewRunner(root, 0)
	p := Pipeline{Stages: []Stage{
		{Name: "pwd", Command: "pwd", Fast: true, Timeout: time.Minute},
	}}
	res := r.Run(context.Background(), p, false)
	if !res.Passed {
		t.Fatal("pwd stage should pass")
	}
	resolved, _ := filepath.EvalSymlinks(root)
	if got := res.Stages[0].Output; got != root && got != resolved {
		t.Errorf("stage cwd = %q, want %q", got, root)
	}
}

func TestTriggerFastOnlySkipsSlow(t *testing.T) {
	root := t.TempDir()
	trig := NewTrigger(NewRunner(root, 0), Pipeline{Stages: []Stage{
		{Name: "fast", Command: "true", Fast: true, Timeout: time.Minute},
		{Name: "slow", Command: "false", Fast: false, Timeout: time.Minute},
	}})
	obs, ran := trig.Verify(context.Background(), true)
	if !ran {
		t.Fatal("fast verify should report it ran")
	}
	if !strings.Contains(obs, "PASS") {
		t.Errorf("fast-only run should pass (slow failing stage skipped); got: %q", obs)
	}
}

func TestTriggerFullRunsAllStages(t *testing.T) {
	root := t.TempDir()
	trig := NewTrigger(NewRunner(root, 0), Pipeline{Stages: []Stage{
		{Name: "fast", Command: "true", Fast: true, Timeout: time.Minute},
		{Name: "slow", Command: "false", Fast: false, Timeout: time.Minute},
	}})
	obs, ran := trig.Verify(context.Background(), false)
	if !ran {
		t.Fatal("full verify should report it ran")
	}
	if !strings.Contains(obs, "FAIL") {
		t.Errorf("full run must include the failing slow stage; got: %q", obs)
	}
}

func TestTriggerFastOnlyNoFastStages(t *testing.T) {
	root := t.TempDir()
	trig := NewTrigger(NewRunner(root, 0), Pipeline{Stages: []Stage{
		{Name: "test", Command: "false", Fast: false, Timeout: time.Minute},
	}})
	if _, ran := trig.Verify(context.Background(), true); ran {
		t.Error("fast verify with no fast stages must report it did not run")
	}
}

func TestRunnerTimeout(t *testing.T) {
	root := t.TempDir()
	r := NewRunner(root, 0)
	p := Pipeline{Stages: []Stage{
		{Name: "slow", Command: "sleep 5", Fast: true, Timeout: 100 * time.Millisecond},
	}}
	res := r.Run(context.Background(), p, false)
	if res.Passed {
		t.Error("timed-out stage must fail")
	}
}
