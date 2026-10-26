package floatplane

import (
	"testing"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/state"
)

func TestScheduleNextSyncQueuesWhileRootSyncIsRunning(t *testing.T) {
	t.Parallel()

	service := New(&config.Config{PollInterval: 10}, api.NewClient(), state.New())
	service.current = &queuedJob{job: RootSync{Reason: "startup"}}
	from := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)

	service.scheduleNextSync(from)

	if got, want := len(service.queue), 1; got != want {
		t.Fatalf("queued job count = %d, want %d", got, want)
	}
	queued := service.queue[0]
	if got, want := queued.job.Kind(), KindSyncRoot; got != want {
		t.Errorf("queued job kind = %q, want %q", got, want)
	}
	if got, want := queued.snapshot.RunAt, from.Add(10*time.Second); !got.Equal(want) {
		t.Errorf("scheduled time = %s, want %s", got, want)
	}
}
