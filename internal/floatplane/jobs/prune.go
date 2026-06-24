package jobs

import (
	"fmt"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
)

type PruneSpec struct {
	Creator string
	Channel string
	Dir     string
	Cutoff  time.Time
}

func (j PruneSpec) Kind() runtime.Kind {
	return runtime.KindPrune
}

func (j PruneSpec) Target() string {
	if j.Channel != "" && j.Channel != j.Creator {
		return j.Creator + " / " + j.Channel
	}
	return j.Creator
}

func (j PruneSpec) Run(ctx *runtime.Context) error {
	removed, err := ctx.Runtime().RunPrune(j.Creator, j.Channel, j.Dir, j.Cutoff)
	if err != nil {
		return err
	}
	if removed == 0 {
		ctx.SetDetails("no expired videos")
		return runtime.Skipf("no expired videos")
	}
	ctx.SetDetails(fmt.Sprintf("removed %d expired video(s)", removed))
	return nil
}
