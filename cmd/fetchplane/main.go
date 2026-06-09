package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Timendus/fetchplane/internal/api"
	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/downloader"
	"github.com/Timendus/fetchplane/internal/state"
)

const pageSize = 20

type preparedCreator struct {
	Info     *api.CreatorInfo
	Channels map[string]api.ChannelInfo
}

func main() {
	daemon := flag.Bool("daemon", false, "run continuously, polling on FP_POLL_INTERVAL (default 300s)")
	debug := flag.Bool("debug", false, "log HTTP requests and responses")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if !cfg.Since.IsZero() {
		log.Printf("only downloading videos released after %s", cfg.Since.Format(time.RFC3339))
	}

	st, err := state.Load(cfg.StateFile)
	if err != nil {
		log.Fatalf("state error: %v", err)
	}

	client, err := api.NewClient()
	if err != nil {
		log.Fatalf("client error: %v", err)
	}
	client.SetDebug(*debug)

	if *daemon {
		runDaemon(cfg, client, st)
		return
	}

	if err := run(cfg, client, st); err != nil {
		log.Fatalf("error: %v", err)
	}
	if err := st.Save(cfg.StateFile); err != nil {
		log.Fatalf("state save error: %v", err)
	}
}

func runDaemon(cfg *config.Config, client *api.Client, st *state.State) {
	log.Printf("daemon mode - polling every %ds", cfg.PollInterval)
	ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
	defer ticker.Stop()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	for {
		if err := run(cfg, client, st); err != nil {
			log.Printf("run error: %v", err)
		}
		if err := st.Save(cfg.StateFile); err != nil {
			log.Printf("state save error: %v", err)
		}

		select {
		case <-ticker.C:
		case sig := <-signals:
			log.Printf("received %s, shutting down", sig)
			return
		}
	}
}

func run(cfg *config.Config, client *api.Client, st *state.State) error {
	if err := ensureAuthenticated(cfg, client, st); err != nil {
		return err
	}

	subs, err := client.GetSubscriptions()
	if err != nil {
		return err
	}
	log.Printf("found %d subscription(s)", len(subs))

	for _, sub := range subs {
		if sub.Creator == "" {
			log.Printf("skipping subscription with empty creator ID")
			continue
		}

		creator, err := client.GetCreatorInfo(sub.Creator)
		if err != nil {
			log.Printf("  error loading creator %s: %v", sub.Creator, err)
			continue
		}
		log.Printf("checking creator: %s", creator.Title)

		prepared, err := prepareCreator(cfg, client, creator)
		if err != nil {
			log.Printf("  error preparing %s: %v", creator.Title, err)
			continue
		}

		if err := processCreator(cfg, client, st, prepared); err != nil {
			log.Printf("  error processing %s: %v", creator.Title, err)
		}
	}

	return nil
}

func ensureAuthenticated(cfg *config.Config, client *api.Client, st *state.State) error {
	if st.Auth.ClientID != "" && st.Auth.ClientID != cfg.OAuthClientID {
		log.Printf("OAuth client ID changed from %s to %s; clearing stored tokens", st.Auth.ClientID, cfg.OAuthClientID)
		st.ClearAuth()
	}

	if accessToken := st.AccessToken(); accessToken != "" {
		expiresAt := st.TokenExpiresAt()
		if time.Now().Add(60 * time.Second).Before(expiresAt) {
			client.SetAccessToken(accessToken)
			return nil
		}
	}

	if refreshToken := st.RefreshToken(); refreshToken != "" {
		token, err := client.RefreshToken(cfg.OAuthClientID, refreshToken)
		if err == nil {
			if err := saveToken(cfg, client, st, token); err != nil {
				return err
			}
			return nil
		}
		log.Printf("refresh failed, starting new device login: %v", err)
		st.ClearAuth()
	}

	deviceAuth, err := client.BeginDeviceAuthorization(cfg.OAuthClientID, cfg.OAuthScope)
	if err != nil {
		return err
	}

	log.Printf("authorize FetchPlane by visiting %s", firstNonEmpty(deviceAuth.VerificationURIComplete, deviceAuth.VerificationURI))
	log.Printf("enter code: %s", deviceAuth.UserCode)

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		token, oauthErr, err := client.PollDeviceAuthorization(cfg.OAuthClientID, deviceAuth.DeviceCode)
		if err != nil {
			return err
		}
		if token != nil {
			if err := saveToken(cfg, client, st, token); err != nil {
				return err
			}
			return nil
		}
		if oauthErr == nil {
			continue
		}

		switch oauthErr.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		default:
			return fmt.Errorf("device authorization failed: %s", oauthErr.ErrorDescription)
		}
	}

	return fmt.Errorf("device authorization timed out after %ds", deviceAuth.ExpiresIn)
}

func saveToken(cfg *config.Config, client *api.Client, st *state.State, token *api.OAuthToken) error {
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	st.SetAuth(cfg.OAuthClientID, token.TokenType, token.Scope, token.AccessToken, token.RefreshToken, expiresAt)
	client.SetAccessToken(token.AccessToken)
	if err := st.Save(cfg.StateFile); err != nil {
		return fmt.Errorf("state save error after auth: %w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func processCreator(cfg *config.Config, client *api.Client, st *state.State, prepared *preparedCreator) error {
	creator := prepared.Info
	fetchAfter := 0
	incrementalCutoff := st.LatestReleaseTime(creator.ID)

	for {
		posts, err := client.GetBlogPosts(creator.ID, pageSize, fetchAfter)
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
			st.RememberLatestRelease(creator.ID, released)

			// Keep scanning the current page even after hitting older posts because
			// the feed is not guaranteed to be perfectly ordered.
			if !cfg.Since.IsZero() && !released.After(cfg.Since) {
				continue
			}
			done = false

			// The state-based cutoff is softer: only stop once we've reached posts we
			// have already fully processed for this creator.
			if !incrementalCutoff.IsZero() && !released.After(incrementalCutoff) && st.HasAllAttachments(attachments) {
				continue
			}
			done = false

			if len(attachments) == 0 {
				continue // picture or text post, nothing to download
			}

			for _, video := range attachments {
				if st.HasAttachment(video.ID) {
					continue
				}

				if filename, exists := downloader.VideoExists(cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID); exists {
					st.RememberAttachment(creator.ID, post.ID, video.ID, filename, released)
					log.Printf("  skip delivery lookup (exists): %s", filename)
					continue
				}

				thumbnailURL := ""
				if post.Thumbnail != nil {
					thumbnailURL = post.Thumbnail.Path
				}
				if _, _, err := downloader.SaveThumbnail(client.GetMedia, cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, thumbnailURL); err != nil {
					log.Printf("  error saving thumbnail for %q: %v", post.Title, err)
				}
				if _, _, err := downloader.SaveNFO(cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, creator.Title, post.Text, released); err != nil {
					log.Printf("  error saving nfo for %q: %v", post.Title, err)
				}

				videoInfo, err := client.GetVideoInfo(video.ID)
				if err != nil {
					log.Printf("  error getting video info for %q: %v", post.Title, err)
				} else {
					subtitles := make([]downloader.SubtitleTrack, 0, len(videoInfo.TextTracks))
					for _, track := range videoInfo.TextTracks {
						if track.Kind != "captions" {
							continue
						}
						subtitles = append(subtitles, downloader.SubtitleTrack{
							URL:      track.Src,
							Language: track.Language,
						})
					}
					if err := downloader.SaveSubtitles(client.GetPublic, cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, subtitles); err != nil {
						log.Printf("  error saving subtitles for %q: %v", post.Title, err)
					}
				}

				dlURL, err := client.GetDeliveryURL(video.ID)
				if err != nil {
					log.Printf("  error getting delivery URL for %q: %v", post.Title, err)
					continue
				}

				filename, _, err := downloader.Download(client.GetMedia, cfg.DownloadDir, creator.Title, channelTitle, post.Title, video.Title, video.ID, dlURL)
				if err != nil {
					log.Printf("  error downloading %q: %v", post.Title, err)
					continue
				}
				st.RememberAttachment(creator.ID, post.ID, video.ID, filename, released)
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

func prepareCreator(cfg *config.Config, client *api.Client, creator *api.CreatorInfo) (*preparedCreator, error) {
	prepared := &preparedCreator{
		Info:     creator,
		Channels: map[string]api.ChannelInfo{},
	}

	if err := prepareCreatorLibrary(cfg, client, creator); err != nil {
		return nil, err
	}

	if creator.URLName == "" {
		return prepared, nil
	}

	namedCreator, err := client.GetNamedCreator(creator.URLName)
	if err != nil {
		log.Printf("  named creator lookup failed for %s: %v", creator.Title, err)
		return prepared, nil
	}

	for _, channel := range namedCreator.Channels {
		if channel.ID != "" {
			prepared.Channels[channel.ID] = channel
		}
		if err := prepareChannelLibrary(cfg, client, creator.Title, channel); err != nil {
			log.Printf("  error preparing channel %s: %v", channel.Title, err)
		}
	}

	return prepared, nil
}

func prepareCreatorLibrary(cfg *config.Config, client *api.Client, creator *api.CreatorInfo) error {
	if creator == nil {
		return nil
	}
	if _, err := downloader.EnsureLibraryDir(cfg.DownloadDir, creator.Title, ""); err != nil {
		return err
	}
	if creator.Cover != nil && creator.Cover.Path != "" {
		if _, _, err := downloader.SaveLibraryArtwork(client.GetMedia, cfg.DownloadDir, creator.Title, "", "fanart", creator.Cover.Path); err != nil {
			log.Printf("  error saving creator cover for %s: %v", creator.Title, err)
		}
	}
	if creator.Icon != nil && creator.Icon.Path != "" {
		if _, _, err := downloader.SaveLibraryArtwork(client.GetMedia, cfg.DownloadDir, creator.Title, "", "folder", creator.Icon.Path); err != nil {
			log.Printf("  error saving creator icon for %s: %v", creator.Title, err)
		}
	}
	return nil
}

func prepareChannelLibrary(cfg *config.Config, client *api.Client, creatorTitle string, channel api.ChannelInfo) error {
	if channel.Title == "" || channel.Title == creatorTitle {
		return nil
	}
	if _, err := downloader.EnsureLibraryDir(cfg.DownloadDir, creatorTitle, channel.Title); err != nil {
		return err
	}
	if channel.Cover != nil && channel.Cover.Path != "" {
		if _, _, err := downloader.SaveLibraryArtwork(client.GetMedia, cfg.DownloadDir, creatorTitle, channel.Title, "fanart", channel.Cover.Path); err != nil {
			log.Printf("  error saving channel cover for %s: %v", channel.Title, err)
		}
	}
	if channel.Icon != nil && channel.Icon.Path != "" {
		if _, _, err := downloader.SaveLibraryArtwork(client.GetMedia, cfg.DownloadDir, creatorTitle, channel.Title, "folder", channel.Icon.Path); err != nil {
			log.Printf("  error saving channel poster for %s: %v", channel.Title, err)
		}
	}
	return nil
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
