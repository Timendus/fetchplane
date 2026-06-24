package floatplane

import (
	"log"
	"time"
)

func (s *Service) run() {
	for {
		nextJob, wait := s.takeNextJob()
		if nextJob == nil {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-s.wakeCh:
				if !timer.Stop() {
					<-timer.C
				}
			}
			continue
		}

		s.runJob(nextJob)
	}
}

func (s *Service) takeNextJob() (*queuedJob, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return nil, time.Hour
	}

	now := time.Now()
	nextIdx := -1
	nextRunAt := time.Time{}
	for idx, queued := range s.queue {
		if nextIdx == -1 || queued.snapshot.RunAt.Before(nextRunAt) || (queued.snapshot.RunAt.Equal(nextRunAt) && queued.snapshot.ID < s.queue[nextIdx].snapshot.ID) {
			nextIdx = idx
			nextRunAt = queued.snapshot.RunAt
		}
	}

	if nextIdx == -1 {
		return nil, time.Hour
	}

	if nextRunAt.After(now) {
		return nil, time.Until(nextRunAt)
	}

	nextJob := s.queue[nextIdx]
	s.queue = append(s.queue[:nextIdx], s.queue[nextIdx+1:]...)
	nextJob.snapshot.Status = StatusRunning
	nextJob.snapshot.StartedAt = now
	s.current = nextJob
	s.notifyLocked()
	return nextJob, 0
}

func (s *Service) runJob(nextJob *queuedJob) {
	log.Printf("job %d started: %s %s", nextJob.snapshot.ID, nextJob.snapshot.Kind, nextJob.snapshot.Target)

	ctx := newJobContext(&nextJob.snapshot, func(child Job, runAt time.Time) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.enqueueJobLocked(child, runAt, nextJob.snapshot.ID)
	})

	err := nextJob.job.Run(s, ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err == nil {
		nextJob.snapshot.Status = StatusSucceeded
	} else {
		nextJob.snapshot.Error = err.Error()
		if statusErr, ok := err.(*StatusError); ok {
			nextJob.snapshot.Status = statusErr.Status
		} else {
			nextJob.snapshot.Status = StatusFailed
		}
	}
	nextJob.snapshot.FinishedAt = time.Now()

	if nextJob.snapshot.Status == StatusFailed {
		log.Printf("job %d failed: %s", nextJob.snapshot.ID, nextJob.snapshot.Error)
	} else {
		log.Printf("job %d finished: %s", nextJob.snapshot.ID, nextJob.snapshot.Status)
	}

	s.current = nil
	s.history = append([]JobSnapshot{nextJob.snapshot}, s.history...)
	if len(s.history) > maxRecentJobs {
		s.history = s.history[:maxRecentJobs]
	}
	s.notifyLocked()
}

func (s *Service) enqueueJobLocked(job Job, runAt time.Time, parentID int64) *queuedJob {
	s.nextJobID++
	queued := &queuedJob{
		snapshot: JobSnapshot{
			ID:       s.nextJobID,
			Kind:     job.Kind(),
			Target:   job.Target(),
			Status:   StatusQueued,
			QueuedAt: time.Now(),
			RunAt:    runAt,
			ParentID: parentID,
		},
		job: job,
	}
	s.queue = append(s.queue, queued)
	s.notifyLocked()
	s.wake()
	return queued
}
