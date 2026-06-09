package sync

import (
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane"
	"github.com/Timendus/fetchplane/internal/library"
)

const pageSize = 20

func (s *Service) processCreator(prepared *preparedCreator) error {
	creator := prepared.Info
	fetchAfter := 0
	incrementalCutoff := s.latestReleaseTime(creator.ID)
	now := time.Now()
	downloadCutoff, hasDownloadCutoff := s.downloadCutoff(now)

	for {
		posts, err := s.client.GetBlogPosts(creator.ID, pageSize, fetchAfter)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			break
		}

		pageHasUsefulPosts := false
		for _, post := range posts {
			attachments := post.Attachments()
			channelTitle := resolveChannelTitle(creator.Title, post.Channel, prepared.Channels)

			released, err := parseReleaseDate(post.ReleaseDate)
			if err != nil {
				log.Printf("  could not parse release date %q for %q, skipping", post.ReleaseDate, post.Title)
				continue
			}
			s.rememberLatestRelease(creator.ID, released)

			if len(attachments) == 0 {
				continue
			}

			postHasVideoFiles := allVideoFilesExist(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, attachments)
			postWithinDownloadWindow := !hasDownloadCutoff || released.After(downloadCutoff)

			// During a single daemon lifetime, stop revisiting older pages once we've
			// already seen them and all of their videos are present on disk.
			if !incrementalCutoff.IsZero() && !released.After(incrementalCutoff) && postHasVideoFiles {
				continue
			}

			// On restart, the library on disk becomes the source of truth. Old posts
			// without local videos are skipped instead of pulling backlog.
			if !postWithinDownloadWindow && !postHasVideoFiles {
				continue
			}

			pageHasUsefulPosts = true
			for _, video := range attachments {
				filename, exists := library.VideoExists(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID)
				if !exists {
					if !postWithinDownloadWindow {
						continue
					}
					dlURL, err := s.client.GetDeliveryURL(video.ID)
					if err != nil {
						log.Printf("  error getting delivery URL for %q: %v", post.Title, err)
						continue
					}

					filename, _, err = library.Download(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, dlURL)
					if err != nil {
						log.Printf("  error downloading %q: %v", post.Title, err)
						continue
					}
				} else {
					log.Printf("  video exists: %s", filename)
				}

				if err := s.ensureVideoSidecars(creator.Title, channelTitle, post, video, released); err != nil {
					log.Printf("  error saving metadata for %q: %v", post.Title, err)
				}
			}
		}

		if !pageHasUsefulPosts || len(posts) < pageSize {
			break
		}
		fetchAfter += len(posts)
	}

	return nil
}

func (s *Service) ensureVideoSidecars(creatorTitle, channelTitle string, post floatplane.BlogPost, video floatplane.VideoAttachment, released time.Time) error {
	thumbnailURL := ""
	if post.Thumbnail != nil {
		thumbnailURL = post.Thumbnail.Path
	}
	if _, _, err := library.SaveThumbnail(s.client.GetMedia, s.cfg.DownloadDir, creatorTitle, channelTitle, post.Title, video.Title, video.ID, thumbnailURL); err != nil {
		log.Printf("  error saving thumbnail for %q: %v", post.Title, err)
	}
	if _, _, err := library.SaveNFO(s.cfg.DownloadDir, creatorTitle, channelTitle, post.Title, video.Title, video.ID, creatorTitle, post.Text, released); err != nil {
		log.Printf("  error saving nfo for %q: %v", post.Title, err)
	}

	videoInfo, err := s.client.GetVideoInfo(video.ID)
	if err != nil {
		return err
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
	return library.SaveSubtitles(s.client.GetPublic, s.cfg.DownloadDir, creatorTitle, channelTitle, post.Title, video.Title, video.ID, subtitles)
}

func allVideoFilesExist(downloadDir, creatorTitle, channelTitle, postTitle string, attachments []floatplane.VideoAttachment) bool {
	for _, video := range attachments {
		if _, exists := library.VideoExists(downloadDir, creatorTitle, channelTitle, postTitle, video.Title, video.ID); !exists {
			return false
		}
	}
	return true
}

func parseReleaseDate(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04:05.000Z", value)
}

func resolveChannelTitle(creatorTitle string, channel floatplane.ChannelRef, prepared map[string]floatplane.ChannelInfo) string {
	if info, ok := prepared[channel.ID]; ok && info.Title != "" {
		return info.Title
	}
	if channel.Title != "" {
		return channel.Title
	}
	return creatorTitle
}
