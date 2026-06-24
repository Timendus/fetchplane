package jobs

import "github.com/Timendus/fetchplane/internal/floatplane/runtime"

func (j AuthWait) Kind() runtime.Kind {
	return runtime.KindAuthWait
}

func (j AuthWait) Target() string {
	return "login"
}

func (j AuthWait) Run(ctx *runtime.Context) error {
	ctx.SetDetails("waiting for user approval")
	return ctx.Runtime().WaitForPendingAuth()
}

type AuthWait struct{}
