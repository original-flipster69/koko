package verify

import "context"

type Trigger struct {
	runner   *Runner
	pipeline Pipeline
}

func NewTrigger(runner *Runner, p Pipeline) *Trigger {
	return &Trigger{runner: runner, pipeline: p}
}

func (t *Trigger) VerifyFast(ctx context.Context) (string, bool) {
	res := t.runner.Run(ctx, t.pipeline, t.pipeline.hasFastStage())
	return res.Observation(), true
}
