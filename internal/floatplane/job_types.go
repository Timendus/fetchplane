package floatplane

import (
	"fmt"
	"time"
)

type Kind string

const (
	KindSyncRoot    Kind = "sync.subscriptions"
	KindCreatorSync Kind = "sync.creator"
	KindPrune       Kind = "prune.channel"
	KindDownload    Kind = "video.download"
	KindAuthWait    Kind = "auth.login"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
)

type JobSnapshot struct {
	ID            int64     `json:"id"`
	Kind          Kind      `json:"kind"`
	Target        string    `json:"target"`
	Status        Status    `json:"status"`
	QueuedAt      time.Time `json:"queuedAt"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt"`
	Details       string    `json:"details"`
	Error         string    `json:"error"`
	Spawned       int       `json:"spawned"`
	ThumbnailPath string    `json:"thumbnailPath"`
	RunAt         time.Time `json:"runAt"`
	ParentID      int64     `json:"parentId"`
}

type Job interface {
	Kind() Kind
	Target() string
	Run(*Service, *jobContext) error
}

type StatusError struct {
	Status Status
	Msg    string
}

func (e *StatusError) Error() string {
	return e.Msg
}

func Skipf(format string, args ...any) error {
	return &StatusError{
		Status: StatusSkipped,
		Msg:    fmt.Sprintf(format, args...),
	}
}
