package floatplane

import (
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/floatplane/jobs"
	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
	"github.com/Timendus/fetchplane/internal/library"
)

func (s *Service) prepareCreator(creator *api.CreatorInfo) (*preparedCreator, error) {
	prepared := &preparedCreator{
		Info:     creator,
		Channels: map[string]api.ChannelInfo{},
	}

	if err := s.prepareCreatorLibrary(creator); err != nil {
		return nil, err
	}

	if creator.URLName == "" {
		return prepared, nil
	}

	namedCreator, err := s.client.GetNamedCreator(creator.URLName)
	if err != nil {
		log.Printf("  named creator lookup failed for %s: %v", creator.Title, err)
		return prepared, nil
	}

	for _, channel := range namedCreator.Channels {
		if channel.ID != "" {
			prepared.Channels[channel.ID] = channel
		}
		if err := s.prepareChannelLibrary(creator.Title, channel); err != nil {
			log.Printf("  error preparing channel %s: %v", channel.Title, err)
		}
	}

	return prepared, nil
}

func (s *Service) prepareCreatorLibrary(creator *api.CreatorInfo) error {
	if creator == nil {
		return nil
	}
	if _, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creator.Title, ""); err != nil {
		return err
	}
	if creator.Cover != nil && creator.Cover.Path != "" {
		if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, "", "fanart", creator.Cover.Path); err != nil {
			log.Printf("  error saving creator cover for %s: %v", creator.Title, err)
		}
	}
	if creator.Icon != nil && creator.Icon.Path != "" {
		if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, "", "folder", creator.Icon.Path); err != nil {
			log.Printf("  error saving creator icon for %s: %v", creator.Title, err)
		}
	}
	return nil
}

func (s *Service) prepareChannelLibrary(creatorTitle string, channel api.ChannelInfo) error {
	if channel.Title == "" || channel.Title == creatorTitle {
		return nil
	}
	if _, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creatorTitle, channel.Title); err != nil {
		return err
	}
	if channel.Cover != nil && channel.Cover.Path != "" {
		if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creatorTitle, channel.Title, "fanart", channel.Cover.Path); err != nil {
			log.Printf("  error saving channel cover for %s: %v", channel.Title, err)
		}
	}
	if channel.Icon != nil && channel.Icon.Path != "" {
		if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creatorTitle, channel.Title, "folder", channel.Icon.Path); err != nil {
			log.Printf("  error saving channel poster for %s: %v", channel.Title, err)
		}
	}
	return nil
}

func (s *Service) buildPruneJobs(prepared *preparedCreator) []runtime.Job {
	if s.cfg.RemoveOlderThanDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -s.cfg.RemoveOlderThanDays)

	rootDir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, prepared.Info.Title, "")
	if err != nil {
		log.Printf("  error preparing prune dir for %s: %v", prepared.Info.Title, err)
		return nil
	}

	result := []runtime.Job{jobs.PruneSpec{
		Creator: prepared.Info.Title,
		Channel: prepared.Info.Title,
		Dir:     rootDir,
		Cutoff:  cutoff,
	}}

	for _, channel := range prepared.Channels {
		if channel.Title == "" || channel.Title == prepared.Info.Title {
			continue
		}

		dir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, prepared.Info.Title, channel.Title)
		if err != nil {
			log.Printf("  error preparing prune dir for channel %s: %v", channel.Title, err)
			continue
		}

		result = append(result, jobs.PruneSpec{
			Creator: prepared.Info.Title,
			Channel: channel.Title,
			Dir:     dir,
			Cutoff:  cutoff,
		})
	}

	return result
}

func (s *Service) RunPrune(creator, channel, dir string, cutoff time.Time) (int, error) {
	removed, err := library.RemoveOlderThan(dir, cutoff)
	if err != nil {
		return 0, err
	}

	if removed > 0 {
		log.Printf("removed %d expired video(s) from %s", removed, dir)
	}
	return removed, nil
}
