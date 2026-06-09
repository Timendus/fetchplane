package sync

import (
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
	"github.com/Timendus/fetchplane/internal/state"
)

type Service struct {
	cfg    *config.Config
	client *floatplane.Client
	state  *state.State
	latest map[string]time.Time
}

type preparedCreator struct {
	Info     *floatplane.CreatorInfo
	Channels map[string]floatplane.ChannelInfo
}

func New(cfg *config.Config, client *floatplane.Client, st *state.State) *Service {
	return &Service{
		cfg:    cfg,
		client: client,
		state:  st,
		latest: map[string]time.Time{},
	}
}

func (s *Service) Run() error {
	if err := s.ensureAuthenticated(); err != nil {
		return err
	}

	subs, err := s.client.GetSubscriptions()
	if err != nil {
		return err
	}
	log.Printf("found %d subscription(s)", len(subs))

	for _, sub := range subs {
		if sub.Creator == "" {
			log.Printf("skipping subscription with empty creator ID")
			continue
		}

		creator, err := s.client.GetCreatorInfo(sub.Creator)
		if err != nil {
			log.Printf("  error loading creator %s: %v", sub.Creator, err)
			continue
		}
		log.Printf("checking creator: %s", creator.Title)

		prepared, err := s.prepareCreator(creator)
		if err != nil {
			log.Printf("  error preparing %s: %v", creator.Title, err)
			continue
		}

		if err := s.removeExpiredVideos(prepared); err != nil {
			log.Printf("  error cleaning %s: %v", creator.Title, err)
		}

		if err := s.processCreator(prepared); err != nil {
			log.Printf("  error processing %s: %v", creator.Title, err)
		}
	}

	return nil
}

func (s *Service) latestReleaseTime(creatorID string) time.Time {
	return s.latest[creatorID]
}

func (s *Service) rememberLatestRelease(creatorID string, released time.Time) {
	if !released.After(s.latest[creatorID]) {
		return
	}
	s.latest[creatorID] = released
}
