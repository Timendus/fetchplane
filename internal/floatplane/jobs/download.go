package jobs

import (
	"path/filepath"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
)

type DownloadSpec struct {
	Creator  string
	Channel  string
	Post     api.BlogPost
	Video    api.VideoAttachment
	Released time.Time
}

func (j DownloadSpec) Kind() runtime.Kind {
	return runtime.KindDownload
}

func (j DownloadSpec) Target() string {
	if j.Video.Title != "" && j.Video.Title != j.Post.Title {
		return j.Post.Title + " - " + j.Video.Title
	}
	return j.Post.Title
}

func (j DownloadSpec) Run(ctx *runtime.Context) error {
	filename, downloaded, thumbnailPath, err := ctx.Runtime().DownloadVideo(j.Creator, j.Channel, j.Post, j.Video, j.Released)
	if err != nil {
		return err
	}
	if !downloaded {
		ctx.SetDetails("video already exists")
		return runtime.Skipf("video already exists")
	}

	ctx.SetDetails(filepath.Base(filename))
	ctx.SetThumbnailPath(thumbnailPath)
	return nil
}
