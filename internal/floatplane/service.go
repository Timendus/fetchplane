package floatplane

import (
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/floatplane/jobs"
	"github.com/Timendus/fetchplane/internal/floatplane/runtime"
	"github.com/Timendus/fetchplane/internal/state"
)

const maxRecentJobs = 10

type Service struct {
	cfg    *config.Config
	client *api.Client

	mu           sync.RWMutex
	state        *state.State
	latest       map[string]time.Time
	pendingAuth  *PendingAuth
	lastAuthErr  string
	queue        []*queuedJob
	current      *queuedJob
	history      []runtime.Snapshot
	nextJobID    int64
	subscribers  map[chan struct{}]struct{}
	wakeCh       chan struct{}
	startOnce    sync.Once
	loginStartMu sync.Mutex
}

type preparedCreator struct {
	Info     *api.CreatorInfo
	Channels map[string]api.ChannelInfo
}

type PendingAuth struct {
	URL        string    `json:"url"`
	UserCode   string    `json:"userCode"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastResult string    `json:"lastResult"`
	DeviceCode string
	Interval   time.Duration
}

type Snapshot struct {
	Authenticated  bool               `json:"authenticated"`
	TokenExpiresAt time.Time          `json:"tokenExpiresAt"`
	PendingAuth    *PendingAuth       `json:"pendingAuth,omitempty"`
	LastAuthError  string             `json:"lastAuthError"`
	Jobs           []runtime.Snapshot `json:"jobs"`
}

type queuedJob struct {
	snapshot runtime.Snapshot
	job      runtime.Job
}

func New(cfg *config.Config, client *api.Client, st *state.State) *Service {
	return &Service{
		cfg:         cfg,
		client:      client,
		state:       st,
		latest:      map[string]time.Time{},
		subscribers: map[chan struct{}]struct{}{},
		wakeCh:      make(chan struct{}, 1),
	}
}

func (s *Service) Start() {
	s.startOnce.Do(func() {
		go s.run()
		s.RequestSyncNow("startup")
	})
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending *PendingAuth
	if s.pendingAuth != nil {
		copy := *s.pendingAuth
		copy.DeviceCode = ""
		pending = &copy
	}

	jobSnapshots := make([]runtime.Snapshot, 0, maxRecentJobs)
	if s.current != nil {
		jobSnapshots = append(jobSnapshots, normalizeThumbnailPath(s.cfg.DownloadDir, s.current.snapshot))
	}

	queued := append([]*queuedJob(nil), s.queue...)
	slices.SortFunc(queued, func(a, b *queuedJob) int {
		if a.snapshot.RunAt.Before(b.snapshot.RunAt) {
			return -1
		}
		if a.snapshot.RunAt.After(b.snapshot.RunAt) {
			return 1
		}
		if a.snapshot.ID < b.snapshot.ID {
			return -1
		}
		if a.snapshot.ID > b.snapshot.ID {
			return 1
		}
		return 0
	})

	for _, queuedJob := range queued {
		if len(jobSnapshots) >= maxRecentJobs {
			break
		}
		jobSnapshots = append(jobSnapshots, normalizeThumbnailPath(s.cfg.DownloadDir, queuedJob.snapshot))
	}

	for _, finished := range s.history {
		if len(jobSnapshots) >= maxRecentJobs {
			break
		}
		jobSnapshots = append(jobSnapshots, normalizeThumbnailPath(s.cfg.DownloadDir, finished))
	}

	authenticated := false
	expiresAt := s.state.TokenExpiresAt()
	if accessToken := s.state.AccessToken(); accessToken != "" && time.Now().Before(expiresAt) {
		authenticated = true
	} else if s.state.RefreshToken() != "" {
		authenticated = true
	}

	return Snapshot{
		Authenticated:  authenticated,
		TokenExpiresAt: expiresAt,
		PendingAuth:    pending,
		LastAuthError:  s.lastAuthErr,
		Jobs:           jobSnapshots,
	}
}

func normalizeThumbnailPath(downloadDir string, snapshot runtime.Snapshot) runtime.Snapshot {
	if snapshot.ThumbnailPath == "" {
		return snapshot
	}
	if rel, err := filepath.Rel(downloadDir, snapshot.ThumbnailPath); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		snapshot.ThumbnailPath = filepath.ToSlash(rel)
	} else {
		snapshot.ThumbnailPath = ""
	}
	return snapshot
}

func (s *Service) SubscribeChanges() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}

	return ch, cancel
}

func (s *Service) RequestSyncNow(reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, queued := range s.queue {
		if queued.job.Kind() != runtime.KindSyncRoot {
			continue
		}
		if queued.snapshot.RunAt.After(now) {
			queued.snapshot.RunAt = now
			queued.snapshot.Target = reason
			s.notifyLocked()
			s.wake()
		}
		return true
	}

	if s.current != nil && s.current.job.Kind() == runtime.KindSyncRoot {
		return false
	}

	s.enqueueJobLocked(jobs.RootSync{Reason: reason}, now, 0)
	return true
}

func (s *Service) StartOrReuseLogin(reason string) (string, error) {
	s.loginStartMu.Lock()
	defer s.loginStartMu.Unlock()

	if pending := s.currentPendingAuth(); pending != nil {
		return pending.URL, nil
	}

	deviceAuth, err := s.client.BeginDeviceAuthorization(s.cfg.OAuthClientID, s.cfg.OAuthScope)
	if err != nil {
		return "", err
	}

	url := firstNonEmpty(deviceAuth.VerificationURIComplete, deviceAuth.VerificationURI)
	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	pending := &PendingAuth{
		URL:        url,
		UserCode:   deviceAuth.UserCode,
		ExpiresAt:  time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second),
		DeviceCode: deviceAuth.DeviceCode,
		Interval:   interval,
	}

	s.mu.Lock()
	s.pendingAuth = pending
	s.lastAuthErr = ""
	if s.current == nil || s.current.job.Kind() != runtime.KindAuthWait {
		hasQueuedAuthWait := false
		for _, queued := range s.queue {
			if queued.job.Kind() == runtime.KindAuthWait {
				hasQueuedAuthWait = true
				break
			}
		}
		if !hasQueuedAuthWait {
			s.enqueueJobLocked(jobs.AuthWait{}, time.Now(), 0)
		}
	}
	s.notifyLocked()
	s.mu.Unlock()
	s.wake()

	if reason != "" {
		log.Printf("%s: authorize FetchPlane by visiting %s", reason, url)
		log.Printf("%s: enter code: %s", reason, deviceAuth.UserCode)
	}

	return url, nil
}

func (s *Service) authState() state.AuthState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Auth
}

func (s *Service) tokenExpiresAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.TokenExpiresAt()
}

func (s *Service) refreshToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.RefreshToken()
}

func (s *Service) currentPendingAuth() *PendingAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pendingAuth == nil {
		return nil
	}
	if time.Now().After(s.pendingAuth.ExpiresAt) {
		return nil
	}
	copy := *s.pendingAuth
	return &copy
}

func (s *Service) clearPendingAuth(result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingAuth != nil {
		s.pendingAuth.LastResult = result
	}
	s.pendingAuth = nil
	s.notifyLocked()
}

func (s *Service) setLastAuthError(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAuthErr = message
	s.notifyLocked()
}

func (s *Service) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *Service) notifyLocked() {
	for subscriber := range s.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}
