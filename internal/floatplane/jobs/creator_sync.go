package jobs

import (
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
)

type CreatorSync struct {
	CreatorID string
}

func (j CreatorSync) Kind() runtime.Kind {
	return runtime.KindCreatorSync
}

func (j CreatorSync) Target() string {
	return j.CreatorID
}

func (j CreatorSync) Run(ctx *runtime.Context) error {
	if j.CreatorID == "" {
		return runtime.Skipf("creator ID missing")
	}

	children, details, err := ctx.Runtime().PlanCreatorJobs(j.CreatorID)
	if err != nil {
		return err
	}
	ctx.SetDetails(details)
	for _, child := range children {
		ctx.EnqueueChild(child, time.Now())
	}
	return nil
}
