package floatplane

import (
	"fmt"
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/library"
)

type PruneSpec struct {
	Creator string
	Channel string
	Dir     string
	Cutoff  time.Time
}

func (j PruneSpec) Kind() Kind {
	return KindPrune
}

func (j PruneSpec) Target() string {
	if j.Channel != "" && j.Channel != j.Creator {
		return j.Creator + " / " + j.Channel
	}
	return j.Creator
}

func (j PruneSpec) Run(_ *Service, ctx *jobContext) error {
	removed, err := library.RemoveOlderThan(j.Dir, j.Cutoff)
	if err != nil {
		return err
	}
	if removed == 0 {
		ctx.SetDetails("no expired videos")
		return Skipf("no expired videos")
	}

	log.Printf("removed %d expired video(s) from %s", removed, j.Dir)
	ctx.SetDetails(fmt.Sprintf("removed %d expired video(s)", removed))
	return nil
}
