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

func TestUpdateSettingsReschedulesPendingSync(t *testing.T) {
	t.Parallel()

	service := New(&config.Config{PollInterval: 60}, api.NewClient(), state.New())
	service.scheduleNextSync(time.Now())

	if err := service.UpdateSettings(config.Editable{
		PollInterval:        10,
		MaxDaysToDownload:   7,
		RemoveOlderThanDays: 0,
	}); err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}

	if got, want := service.Settings().PollInterval, 10; got != want {
		t.Errorf("poll interval = %d, want %d", got, want)
	}
	if got, want := len(service.queue), 1; got != want {
		t.Fatalf("queued job count = %d, want %d", got, want)
	}
	remaining := time.Until(service.queue[0].snapshot.RunAt)
	if remaining < 9*time.Second || remaining > 10*time.Second {
		t.Errorf("rescheduled sync is due in %s, want about 10s", remaining)
	}
}
