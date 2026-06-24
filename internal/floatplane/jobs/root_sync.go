package jobs

import (
	"fmt"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
)

type RootSync struct {
	Reason string
}

func (j RootSync) Kind() runtime.Kind {
	return runtime.KindSyncRoot
}

func (j RootSync) Target() string {
	return j.Reason
}

func (j RootSync) Run(ctx *runtime.Context) error {
	authenticated, err := ctx.Runtime().EnsureAuthenticated(j.Reason == "startup")
	if err != nil {
		return err
	}
	if !authenticated {
		return runtime.Skipf("authentication pending")
	}

	ctx.Runtime().ScheduleNextSync(time.Now())

	subs, err := ctx.Runtime().GetSubscriptions()
	if err != nil {
		return err
	}

	ctx.SetDetails(fmt.Sprintf("found %d subscription(s)", len(subs)))
	for _, sub := range subs {
		if sub.Creator == "" {
			continue
		}
		ctx.EnqueueChild(CreatorSync{CreatorID: sub.Creator}, time.Now())
	}

	return nil
}
