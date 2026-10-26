package floatplane

import (
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/library"
)

const pageSize = 20

type CreatorSync struct {
	CreatorID string
}

func (j CreatorSync) Kind() Kind {
	return KindCreatorSync
}

func (j CreatorSync) Target() string {
	return j.CreatorID
}

func (j CreatorSync) Run(s *Service, ctx *jobContext) error {
	if j.CreatorID == "" {
		return Skipf("creator ID missing")
	}
	settings := s.Settings()

	creator, err := s.client.GetCreatorInfo(j.CreatorID)
	if err != nil {
		return err
	}

	channels, err := prepareCreatorLibrary(s, creator)
	if err != nil {
		return err
	}

	ctx.SetDetails("checking " + creator.Title)
	enqueuePruneJobs(s, ctx, creator.Title, channels, settings.RemoveOlderThanDays)

	fetchAfter := 0
	now := time.Now()
	incrementalCutoff := s.latest[creator.ID]
	var downloadCutoff time.Time
	hasDownloadCutoff := settings.MaxDaysToDownload > 0
	if hasDownloadCutoff {
		downloadCutoff = now.AddDate(0, 0, -settings.MaxDaysToDownload)
	}

	for {
		posts, err := s.client.GetBlogPosts(creator.ID, pageSize, fetchAfter)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			return nil
		}

		pageHasUsefulPosts := false
		for _, post := range posts {
			released, ok := parsePostRelease(post)
			if !ok {
				continue
			}
			if released.After(s.latest[creator.ID]) {
				s.latest[creator.ID] = released
			}

			// The creator listing only includes post metadata. Floatplane no
			// longer returns videoAttachments there, so retrieve the complete
			// post before looking for videos to download.
			fullPost, err := s.client.GetBlogPost(post.ID)
			if err != nil {
				return err
			}
			post = *fullPost

			attachments := post.Attachments()
			if len(attachments) == 0 {
				continue
			}

			channelTitle := resolveChannelTitle(creator.Title, post.Channel, channels)
			postHasVideoFiles := allVideoFilesExist(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, attachments)
			postWithinDownloadWindow := !hasDownloadCutoff || released.After(downloadCutoff)

			if !incrementalCutoff.IsZero() && !released.After(incrementalCutoff) && postHasVideoFiles {
				continue
			}
			if !postWithinDownloadWindow && !postHasVideoFiles {
				continue
			}

			pageHasUsefulPosts = true
			for _, video := range attachments {
				filename, exists := library.VideoExists(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID)
				if !exists {
					if postWithinDownloadWindow {
						ctx.EnqueueChild(DownloadSpec{
							Creator:  creator.Title,
							Channel:  channelTitle,
							Post:     post,
							Video:    video,
							Released: released,
						}, time.Now())
					}
					continue
				}

				log.Printf("  video exists: %s", filename)
				if _, err := saveVideoSidecars(s, creator.Title, channelTitle, post, video, released); err != nil {
					log.Printf("  error saving metadata for %q: %v", post.Title, err)
				}
			}
		}

		if !pageHasUsefulPosts || len(posts) < pageSize {
			return nil
		}
		fetchAfter += len(posts)
	}
}

func prepareCreatorLibrary(s *Service, creator *api.CreatorInfo) (map[string]api.ChannelInfo, error) {
	channels := map[string]api.ChannelInfo{}
	if creator == nil {
		return channels, nil
	}

	if _, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creator.Title, ""); err != nil {
		return nil, err
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

	if creator.URLName == "" {
		return channels, nil
	}

	namedCreator, err := s.client.GetNamedCreator(creator.URLName)
	if err != nil {
		log.Printf("  named creator lookup failed for %s: %v", creator.Title, err)
		return channels, nil
	}

	for _, channel := range namedCreator.Channels {
		if channel.ID != "" {
			channels[channel.ID] = channel
		}
		if channel.Title == "" || channel.Title == creator.Title {
			continue
		}
		if _, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creator.Title, channel.Title); err != nil {
			log.Printf("  error preparing channel %s: %v", channel.Title, err)
			continue
		}
		if channel.Cover != nil && channel.Cover.Path != "" {
			if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, channel.Title, "fanart", channel.Cover.Path); err != nil {
				log.Printf("  error saving channel cover for %s: %v", channel.Title, err)
			}
		}
		if channel.Icon != nil && channel.Icon.Path != "" {
			if _, _, err := library.SaveLibraryArtwork(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, channel.Title, "folder", channel.Icon.Path); err != nil {
				log.Printf("  error saving channel poster for %s: %v", channel.Title, err)
			}
		}
	}

	return channels, nil
}

func enqueuePruneJobs(s *Service, ctx *jobContext, creatorTitle string, channels map[string]api.ChannelInfo, removeOlderThanDays int) {
	if removeOlderThanDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -removeOlderThanDays)

	rootDir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creatorTitle, "")
	if err != nil {
		log.Printf("  error preparing prune dir for %s: %v", creatorTitle, err)
		return
	}
	ctx.EnqueueChild(PruneSpec{
		Creator: creatorTitle,
		Channel: creatorTitle,
		Dir:     rootDir,
		Cutoff:  cutoff,
	}, time.Now())

	for _, channel := range channels {
		if channel.Title == "" || channel.Title == creatorTitle {
			continue
		}
		dir, err := library.EnsureLibraryDir(s.cfg.DownloadDir, creatorTitle, channel.Title)
		if err != nil {
			log.Printf("  error preparing prune dir for channel %s: %v", channel.Title, err)
			continue
		}
		ctx.EnqueueChild(PruneSpec{
			Creator: creatorTitle,
			Channel: channel.Title,
			Dir:     dir,
			Cutoff:  cutoff,
		}, time.Now())
	}
}

func parsePostRelease(post api.BlogPost) (time.Time, bool) {
	released, err := time.Parse(time.RFC3339, post.ReleaseDate)
	if err == nil {
		return released, true
	}
	released, err = time.Parse("2006-01-02T15:04:05.000Z", post.ReleaseDate)
	if err == nil {
		return released, true
	}
	log.Printf("  could not parse release date %q for %q, skipping", post.ReleaseDate, post.Title)
	return time.Time{}, false
}

func allVideoFilesExist(downloadDir, creatorTitle, channelTitle, postTitle string, attachments []api.VideoAttachment) bool {
	for _, video := range attachments {
		if _, exists := library.VideoExists(downloadDir, creatorTitle, channelTitle, postTitle, video.Title, video.ID); !exists {
			return false
		}
	}
	return true
}

func resolveChannelTitle(creatorTitle string, channel api.ChannelRef, channels map[string]api.ChannelInfo) string {
	if info, ok := channels[channel.ID]; ok && info.Title != "" {
		return info.Title
	}
	if channel.Title != "" {
		return channel.Title
	}
	return creatorTitle
}

func saveVideoSidecars(s *Service, creatorTitle, channelTitle string, post api.BlogPost, video api.VideoAttachment, released time.Time) (string, error) {
	videoInfo, err := s.client.GetVideoInfo(video.ID)
	if err != nil {
		return "", err
	}

	subtitles := make([]library.SubtitleTrack, 0, len(videoInfo.TextTracks))
	for _, track := range videoInfo.TextTracks {
		if track.Kind != "captions" {
			continue
		}
		subtitles = append(subtitles, library.SubtitleTrack{
			URL:      track.Src,
			Language: track.Language,
		})
	}

	thumbnailURL := ""
	if post.Thumbnail != nil {
		thumbnailURL = post.Thumbnail.Path
	}

	return library.SaveVideoSidecars(
		s.client.GetMedia,
		s.client.GetPublic,
		s.cfg.DownloadDir,
		creatorTitle,
		channelTitle,
		post.Title,
		video.Title,
		video.ID,
		creatorTitle,
		post.Text,
		released,
		thumbnailURL,
		subtitles,
	)
}
