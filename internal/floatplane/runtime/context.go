package runtime

import (
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
)

type Runtime interface {
	EnsureAuthenticated(allowLoginStart bool) (bool, error)
	ScheduleNextSync(time.Time)
	GetSubscriptions() ([]api.Subscription, error)
	PlanCreatorJobs(creatorID string) ([]Job, string, error)
	RunPrune(creator, channel, dir string, cutoff time.Time) (int, error)
	DownloadVideo(creator, channel string, post api.BlogPost, video api.VideoAttachment, released time.Time) (filename string, downloaded bool, thumbnailPath string, err error)
	WaitForPendingAuth() error
}

type Context struct {
	runtime  Runtime
	snapshot *Snapshot
	enqueue  func(Job, time.Time)
}

func NewContext(runtime Runtime, snapshot *Snapshot, enqueue func(Job, time.Time)) *Context {
	return &Context{
		runtime:  runtime,
		snapshot: snapshot,
		enqueue:  enqueue,
	}
}

func (c *Context) Runtime() Runtime {
	return c.runtime
}

func (c *Context) EnqueueChild(job Job, runAt time.Time) {
	c.snapshot.Spawned++
	c.enqueue(job, runAt)
}

func (c *Context) SetDetails(details string) {
	c.snapshot.Details = details
}

func (c *Context) SetThumbnailPath(path string) {
	c.snapshot.ThumbnailPath = path
}
