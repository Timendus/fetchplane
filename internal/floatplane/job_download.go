package floatplane

import (
	"path/filepath"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/library"
)

type DownloadSpec struct {
	Creator  string
	Channel  string
	Post     api.BlogPost
	Video    api.VideoAttachment
	Released time.Time
}

func (j DownloadSpec) Kind() Kind {
	return KindDownload
}

func (j DownloadSpec) Target() string {
	if j.Video.Title != "" && j.Video.Title != j.Post.Title {
		return j.Post.Title + " - " + j.Video.Title
	}
	return j.Post.Title
}

func (j DownloadSpec) Run(s *Service, ctx *jobContext) error {
	dlURL, err := s.client.GetDeliveryURL(j.Video.ID)
	if err != nil {
		return err
	}

	filename, downloaded, err := library.Download(
		s.client.GetMedia,
		s.cfg.DownloadDir,
		j.Creator,
		j.Channel,
		j.Post.Title,
		j.Video.Title,
		j.Video.ID,
		dlURL,
	)
	if err != nil {
		return err
	}
	if !downloaded {
		ctx.SetDetails("video already exists")
		return Skipf("video already exists")
	}

	thumbnailPath, err := saveVideoSidecars(s, j.Creator, j.Channel, j.Post, j.Video, j.Released)
	if err != nil {
		return err
	}

	ctx.SetDetails(filepath.Base(filename))
	ctx.SetThumbnailPath(thumbnailPath)
	return nil
}
