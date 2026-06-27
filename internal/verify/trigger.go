package verify

import "context"

type Trigger struct {
	runner   *Runner
	pipeline Pipeline
}

func NewTrigger(runner *Runner, p Pipeline) *Trigger {
	return &Trigger{runner: runner, pipeline: p}
}

func (t *Trigger) Verify(ctx context.Context, fastOnly bool) (string, bool) {
	res := t.runner.Run(ctx, t.pipeline, fastOnly)
	if len(res.Stages) == 0 {
		return "", false
	}
	return res.Observation(), true
}
