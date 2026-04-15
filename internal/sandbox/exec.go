package sandbox

import "context"

type execContext struct {
	Ctx context.Context
}

func NewExecContext(ctx context.Context) execContext {
	return execContext{Ctx: ctx}
}
