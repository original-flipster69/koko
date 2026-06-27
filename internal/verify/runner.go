package verify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const defaultOutputCap = 8192

type StageResult struct {
	Name   string
	Passed bool
	Output string
}

type Result struct {
	Stages []StageResult
	Passed bool
}

type Runner struct {
	root      string
	outputCap int
}

func NewRunner(root string, outputCap int) *Runner {
	if outputCap <= 0 {
		outputCap = defaultOutputCap
	}
	return &Runner{root: root, outputCap: outputCap}
}

func (r *Runner) Run(ctx context.Context, p Pipeline, fastOnly bool) Result {
	res := Result{Passed: true}
	for _, st := range p.Stages {
		if fastOnly && !st.Fast {
			continue
		}
		sr := r.runStage(ctx, st)
		res.Stages = append(res.Stages, sr)
		if !sr.Passed {
			res.Passed = false
			break
		}
	}
	return res
}

func (r *Runner) runStage(ctx context.Context, st Stage) StageResult {
	stageCtx, cancel := context.WithTimeout(ctx, st.Timeout)
	defer cancel()

	cmd := exec.CommandContext(stageCtx, "sh", "-c", st.Command)
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	output := truncate(string(out), r.outputCap)

	if stageCtx.Err() == context.DeadlineExceeded {
		return StageResult{
			Name:   st.Name,
			Passed: false,
			Output: output + fmt.Sprintf("\n[stage timed out after %s]", st.Timeout),
		}
	}
	return StageResult{Name: st.Name, Passed: err == nil, Output: output}
}

func (res Result) Observation() string {
	var b strings.Builder
	if res.Passed {
		b.WriteString("verification: PASS\n")
	} else {
		b.WriteString("verification: FAIL\n")
	}
	for _, s := range res.Stages {
		status := "ok"
		if !s.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, status)
	}
	for _, s := range res.Stages {
		if !s.Passed {
			fmt.Fprintf(&b, "\n--- %s output ---\n%s", s.Name, s.Output)
			break
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncate(s string, max int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(output truncated)"
}
