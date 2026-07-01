package webui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	cfg     *config.Config
	service *floatplane.Service
	index   []byte
	static  http.Handler
}

func New(cfg *config.Config, service *floatplane.Service) (*Server, error) {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:     cfg,
		service: service,
		index:   index,
		static:  http.FileServer(http.FS(staticFS)),
	}, nil
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", s.static))
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/sync/now", s.handleCheckNow)
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(s.cfg.DownloadDir))))

	server := &http.Server{
		Addr:              s.cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.index)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := json.Marshal(s.snapshotView())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	changes, cancel := s.service.SubscribeChanges()
	defer cancel()

	send := func() error {
		data, err := json.Marshal(s.snapshotView())
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := send(); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-changes:
			if !ok {
				return
			}
			if err := send(); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	loginURL, err := s.service.StartOrReuseLogin("web")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if loginURL == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, loginURL, http.StatusSeeOther)
}

func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.service.RequestSyncNow("web")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type snapshotView struct {
	Authenticated  bool             `json:"authenticated"`
	TokenExpiresAt string           `json:"tokenExpiresAt"`
	PendingAuth    *pendingAuthView `json:"pendingAuth,omitempty"`
	LastAuthError  string           `json:"lastAuthError"`
	Jobs           []jobView        `json:"jobs"`
}

type pendingAuthView struct {
	URL        string `json:"url"`
	UserCode   string `json:"userCode"`
	ExpiresAt  string `json:"expiresAt"`
	LastResult string `json:"lastResult"`
}

type jobView struct {
	Kind         string `json:"kind"`
	Target       string `json:"target"`
	Status       string `json:"status"`
	QueuedAt     string `json:"queuedAt"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	Details      string `json:"details"`
	Error        string `json:"error"`
	Spawned      int    `json:"spawned"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

func (s *Server) snapshotView() snapshotView {
	snapshot := s.service.Snapshot()

	view := snapshotView{
		Authenticated:  snapshot.Authenticated,
		TokenExpiresAt: formatTime(snapshot.TokenExpiresAt),
		LastAuthError:  snapshot.LastAuthError,
	}

	if snapshot.PendingAuth != nil {
		view.PendingAuth = &pendingAuthView{
			URL:        snapshot.PendingAuth.URL,
			UserCode:   snapshot.PendingAuth.UserCode,
			ExpiresAt:  formatTime(snapshot.PendingAuth.ExpiresAt),
			LastResult: snapshot.PendingAuth.LastResult,
		}
	}

	for _, job := range snapshot.Jobs {
		row := jobView{
			Kind:       string(job.Kind),
			Target:     formatTarget(job),
			Status:     string(job.Status),
			QueuedAt:   formatTime(job.QueuedAt),
			StartedAt:  formatTime(job.StartedAt),
			FinishedAt: formatTime(job.FinishedAt),
			Details:    job.Details,
			Error:      job.Error,
			Spawned:    job.Spawned,
		}
		if job.ThumbnailPath != "" {
			row.ThumbnailURL = "/media/" + escapeMediaPath(job.ThumbnailPath)
		}
		view.Jobs = append(view.Jobs, row)
	}

	return view
}

func formatTarget(job floatplane.JobSnapshot) string {
	if job.Kind == floatplane.KindSyncRoot {
		return "Trigger: " + job.Target
	}
	if job.Kind == floatplane.KindCreatorSync && strings.HasPrefix(job.Details, "checking ") {
		return job.Details[len("checking "):]
	}
	return job.Target
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func escapeMediaPath(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
