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
	incrementalCutoff := s.state.LatestReleaseTime(creator.ID)

	for {
		posts, err := s.client.GetBlogPosts(creator.ID, pageSize, fetchAfter)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			break
		}

		done := true
		for _, post := range posts {
			attachments := post.Attachments()
			channelTitle := resolveChannelTitle(creator.Title, post.Channel, prepared.Channels)

			released, err := parseReleaseDate(post.ReleaseDate)
			if err != nil {
				log.Printf("  could not parse release date %q for %q, skipping", post.ReleaseDate, post.Title)
				continue
			}
			s.state.RememberLatestRelease(creator.ID, released)

			// Keep scanning the current page even after hitting older posts because
			// the feed is not guaranteed to be perfectly ordered.
			if !s.cfg.Since.IsZero() && !released.After(s.cfg.Since) {
				continue
			}
			done = false

			// The state-based cutoff is softer: only stop once we've reached posts we
			// have already fully processed for this creator.
			if !incrementalCutoff.IsZero() && !released.After(incrementalCutoff) && s.state.HasAllAttachments(attachments) {
				continue
			}
			done = false

			if len(attachments) == 0 {
				continue
			}

			for _, video := range attachments {
				if s.state.HasAttachment(video.ID) {
					continue
				}

				if filename, exists := library.VideoExists(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID); exists {
					s.state.RememberAttachment(creator.ID, post.ID, video.ID, filename, released)
					log.Printf("  skip delivery lookup (exists): %s", filename)
					continue
				}

				thumbnailURL := ""
				if post.Thumbnail != nil {
					thumbnailURL = post.Thumbnail.Path
				}
				if _, _, err := library.SaveThumbnail(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, thumbnailURL); err != nil {
					log.Printf("  error saving thumbnail for %q: %v", post.Title, err)
				}
				if _, _, err := library.SaveNFO(s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, creator.Title, post.Text, released); err != nil {
					log.Printf("  error saving nfo for %q: %v", post.Title, err)
				}

				videoInfo, err := s.client.GetVideoInfo(video.ID)
				if err != nil {
					log.Printf("  error getting video info for %q: %v", post.Title, err)
				} else {
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
					if err := library.SaveSubtitles(s.client.GetPublic, s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, subtitles); err != nil {
						log.Printf("  error saving subtitles for %q: %v", post.Title, err)
					}
				}

				dlURL, err := s.client.GetDeliveryURL(video.ID)
				if err != nil {
					log.Printf("  error getting delivery URL for %q: %v", post.Title, err)
					continue
				}

				filename, _, err := library.Download(s.client.GetMedia, s.cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, dlURL)
				if err != nil {
					log.Printf("  error downloading %q: %v", post.Title, err)
					continue
				}
				s.state.RememberAttachment(creator.ID, post.ID, video.ID, filename, released)
			}
		}

		if done || len(posts) < pageSize {
			break
		}
		fetchAfter += len(posts)
	}

	return nil
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
