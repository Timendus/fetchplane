package floatplane

import "time"

type jobContext struct {
	snapshot *JobSnapshot
	enqueue  func(Job, time.Time)
}

func newJobContext(snapshot *JobSnapshot, enqueue func(Job, time.Time)) *jobContext {
	return &jobContext{
		snapshot: snapshot,
		enqueue:  enqueue,
	}
}

func (c *jobContext) EnqueueChild(job Job, runAt time.Time) {
	c.snapshot.Spawned++
	c.enqueue(job, runAt)
}

func (c *jobContext) SetDetails(details string) {
	c.snapshot.Details = details
}

func (c *jobContext) SetThumbnailPath(path string) {
	c.snapshot.ThumbnailPath = path
}
