package floatplane

import (
	"fmt"
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/floatplane/jobs"
	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
	"github.com/Timendus/fetchplane/internal/library"
)

const pageSize = 20

func (s *Service) GetSubscriptions() ([]api.Subscription, error) {
	return s.client.GetSubscriptions()
}

func (s *Service) ScheduleNextSync(from time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runAt := from.Add(time.Duration(s.cfg.PollInterval) * time.Second)
	if s.current != nil && s.current.job.Kind() == runtime.KindSyncRoot {
		return
	}
	for _, queued := range s.queue {
		if queued.job.Kind() == runtime.KindSyncRoot {
			return
		}
	}
	s.enqueueJobLocked(jobs.RootSync{Reason: "scheduled"}, runAt, 0)
}

func (s *Service) PlanCreatorJobs(creatorID string) ([]runtime.Job, string, error) {
	creator, err := s.client.GetCreatorInfo(creatorID)
	if err != nil {
		return nil, "", err
	}
	prepared, err := s.prepareCreator(creator)
	if err != nil {
		return nil, "", err
	}

	result := s.buildPruneJobs(prepared)
	fetchAfter := 0
	now := time.Now()
	incrementalCutoff := s.latest[creator.ID]
	downloadCutoff := time.Time{}
	hasDownloadCutoff := s.cfg.MaxDaysToDownload > 0
	if hasDownloadCutoff {
		downloadCutoff = now.AddDate(0, 0, -s.cfg.MaxDaysToDownload)
	}

	for {
		posts, err := s.client.GetBlogPosts(creator.ID, pageSize, fetchAfter)
		if err != nil {
			return nil, "", err
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
			if released.After(s.latest[creator.ID]) {
				s.latest[creator.ID] = released
			}

			if len(attachments) == 0 {
				continue
			}

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
					if !postWithinDownloadWindow {
						continue
					}

					result = append(result, jobs.DownloadSpec{
						Creator:  creator.Title,
						Channel:  channelTitle,
						Post:     post,
						Video:    video,
						Released: released,
					})
					continue
				}

				log.Printf("  video exists: %s", filename)
				if _, err := s.saveExistingVideoSidecars(creator.Title, channelTitle, post, video, released); err != nil {
					log.Printf("  error saving metadata for %q: %v", post.Title, err)
				}
			}
		}

		if !pageHasUsefulPosts || len(posts) < pageSize {
			break
		}
		fetchAfter += len(posts)
	}

	return result, fmt.Sprintf("checking %s", creator.Title), nil
}

func (s *Service) DownloadVideo(creator, channel string, post api.BlogPost, video api.VideoAttachment, released time.Time) (filename string, downloaded bool, thumbnailPath string, err error) {
	dlURL, err := s.client.GetDeliveryURL(video.ID)
	if err != nil {
		return "", false, "", err
	}

	filename, downloaded, err = library.Download(s.client.GetMedia, s.cfg.DownloadDir, creator, channel, post.Title, video.Title, video.ID, dlURL)
	if err != nil {
		return "", false, "", err
	}

	thumbnailPath, err = s.saveExistingVideoSidecars(creator, channel, post, video, released)
	if err != nil {
		return "", false, "", err
	}

	return filename, downloaded, thumbnailPath, nil
}

func (s *Service) saveExistingVideoSidecars(creatorTitle, channelTitle string, post api.BlogPost, video api.VideoAttachment, released time.Time) (string, error) {
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

func allVideoFilesExist(downloadDir, creatorTitle, channelTitle, postTitle string, attachments []api.VideoAttachment) bool {
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

func resolveChannelTitle(creatorTitle string, channel api.ChannelRef, prepared map[string]api.ChannelInfo) string {
	if info, ok := prepared[channel.ID]; ok && info.Title != "" {
		return info.Title
	}
	if channel.Title != "" {
		return channel.Title
	}
	return creatorTitle
}
