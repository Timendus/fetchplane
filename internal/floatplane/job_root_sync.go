package floatplane

import (
	"fmt"
	"time"
)

type RootSync struct {
	Reason string
}

func (j RootSync) Kind() Kind {
	return KindSyncRoot
}

func (j RootSync) Target() string {
	return j.Reason
}

func (j RootSync) Run(s *Service, ctx *jobContext) error {
	authenticated, err := s.EnsureAuthenticated(j.Reason == "startup")
	if err != nil {
		return err
	}
	if !authenticated {
		return Skipf("authentication pending")
	}

	s.scheduleNextSync(time.Now())

	subs, err := s.client.GetSubscriptions()
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
