package sync

import (
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane"
	"github.com/Timendus/fetchplane/internal/library"
)

func (s *Service) prepareCreator(creator *floatplane.CreatorInfo) (*preparedCreator, error) {
	prepared := &preparedCreator{
		Info:     creator,
		Channels: map[string]floatplane.ChannelInfo{},
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

func (s *Service) prepareCreatorLibrary(creator *floatplane.CreatorInfo) error {
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

func (s *Service) prepareChannelLibrary(creatorTitle string, channel floatplane.ChannelInfo) error {
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

func (s *Service) removeExpiredVideos(prepared *preparedCreator) error {
	cutoff, ok := s.removalCutoff(time.Now())
	if !ok {
		return nil
	}

	rootDir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, prepared.Info.Title, "")
	if err != nil {
		return err
	}
	dirs := []string{rootDir}
	for _, channel := range prepared.Channels {
		if channel.Title == "" || channel.Title == prepared.Info.Title {
			continue
		}
		dir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, prepared.Info.Title, channel.Title)
		if err != nil {
			return err
		}
		dirs = append(dirs, dir)
	}

	for _, dir := range dirs {
		removed, err := library.RemoveOlderThan(dir, cutoff)
		if err != nil {
			return err
		}
		if removed > 0 {
			log.Printf("  removed %d expired video(s) from %s", removed, dir)
		}
	}
	return nil
}
