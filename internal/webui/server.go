package webui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
)

type Server struct {
	cfg     *config.Config
	service *floatplane.Service
	tmpl    *template.Template
}

type pageData struct {
	InitialJSON template.JS
}

func New(cfg *config.Config, service *floatplane.Service) (*Server, error) {
	tmpl, err := template.New("dashboard").Parse(dashboardTemplate)
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:     cfg,
		service: service,
		tmpl:    tmpl,
	}, nil
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
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

	snapshotJSON, err := json.Marshal(s.snapshotView())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, pageData{InitialJSON: template.JS(snapshotJSON)}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
			Target:     job.Target,
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

const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Fetchplane</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f2ea;
      --card: #fffdf8;
      --line: #d8d0c2;
      --text: #2d2418;
      --muted: #776b5d;
      --accent: #a84d1f;
      --accent-soft: #f5d7c8;
      --ok: #2f6f43;
      --warn: #9c5a13;
      --bad: #912f2f;
    }
    body {
      margin: 0;
      background: linear-gradient(180deg, #efe6d7 0%, var(--bg) 35%, #f8f5ef 100%);
      color: var(--text);
      font: 16px/1.45 Georgia, serif;
    }
    main {
      max-width: 1120px;
      margin: 0 auto;
      padding: 24px;
    }
    h1, h2 {
      margin: 0 0 12px;
      font-weight: 600;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
      gap: 16px;
      margin-bottom: 16px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--line);
      border-radius: 14px;
      padding: 18px;
      box-shadow: 0 10px 25px rgba(65, 43, 23, 0.08);
    }
    .muted {
      color: var(--muted);
    }
    .actions {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin-top: 14px;
    }
    button {
      border: 0;
      border-radius: 999px;
      padding: 10px 16px;
      background: var(--accent);
      color: white;
      cursor: pointer;
      font: inherit;
    }
    table {
      width: 100%;
      border-collapse: collapse;
    }
    th, td {
      padding: 10px 8px;
      border-top: 1px solid var(--line);
      vertical-align: top;
      text-align: left;
    }
    th {
      font-size: 13px;
      color: var(--muted);
      letter-spacing: 0.03em;
      text-transform: uppercase;
    }
    .thumb {
      width: 120px;
      height: 68px;
      object-fit: cover;
      border-radius: 8px;
      border: 1px solid var(--line);
      background: #eadfce;
      display: block;
    }
    code {
      font-size: 15px;
      background: rgba(45, 36, 24, 0.08);
      padding: 2px 6px;
      border-radius: 6px;
    }
    .status-ok { color: var(--ok); }
    .status-warn { color: var(--warn); }
    .status-bad { color: var(--bad); }
    a { color: var(--accent); }
  </style>
</head>
<body>
  <main>
    <h1>Fetchplane</h1>
    <div class="grid">
      <section class="card">
        <h2>Authentication</h2>
        <div id="auth-card"></div>
        <div class="actions">
          <form method="post" action="/auth/login" target="_blank">
            <button type="submit">Log in</button>
          </form>
          <form method="post" action="/sync/now">
            <button type="submit">Check now</button>
          </form>
        </div>
      </section>

      <section class="card">
        <h2>Runner</h2>
        <div id="runner-card"></div>
      </section>
    </div>

    <section class="card">
      <h2>Recent Jobs</h2>
      <table>
        <thead>
          <tr>
            <th>Kind</th>
            <th>Target</th>
            <th>Status</th>
            <th>Times</th>
            <th>Details</th>
            <th>Children</th>
            <th>Thumbnail</th>
          </tr>
        </thead>
        <tbody id="jobs-body"></tbody>
      </table>
    </section>
  </main>
  <script>
    const initialSnapshot = {{.InitialJSON}};

    function statusClass(status) {
      if (status === "succeeded") return "status-ok";
      if (status === "failed") return "status-bad";
      if (status === "skipped") return "status-warn";
      return "";
    }

    function render(snapshot) {
      const authCard = document.getElementById("auth-card");
      const runnerCard = document.getElementById("runner-card");
      const jobsBody = document.getElementById("jobs-body");

      let authHtml = "<p>Status: ";
      authHtml += snapshot.authenticated ? "<span class='status-ok'>authenticated</span>" : "<span class='status-warn'>login required</span>";
      authHtml += "</p>";
      if (snapshot.tokenExpiresAt) {
        authHtml += "<p class='muted'>Token expiry: " + escapeHtml(snapshot.tokenExpiresAt) + "</p>";
      }
      if (snapshot.pendingAuth) {
        authHtml += "<p>Current login URL: <a href='" + escapeAttr(snapshot.pendingAuth.url) + "' target='_blank'>" + escapeHtml(snapshot.pendingAuth.url) + "</a></p>";
        authHtml += "<p>Enter code <code>" + escapeHtml(snapshot.pendingAuth.userCode) + "</code></p>";
        authHtml += "<p class='muted'>Expires at " + escapeHtml(snapshot.pendingAuth.expiresAt) + "</p>";
      } else {
        authHtml += "<p class='muted'>No active login session.</p>";
      }
      if (snapshot.lastAuthError) {
        authHtml += "<p class='status-bad'>Last auth error: " + escapeHtml(snapshot.lastAuthError) + "</p>";
      }
      authCard.innerHTML = authHtml;

      const currentJob = snapshot.jobs.find(job => job.status === "running") || snapshot.jobs.find(job => job.status === "queued");
      let runnerHtml = "";
      if (currentJob) {
        runnerHtml += "<p>Current job: <strong>" + escapeHtml(currentJob.kind) + "</strong> " + escapeHtml(currentJob.target || "") + "</p>";
        if (currentJob.details) {
          runnerHtml += "<p class='muted'>" + escapeHtml(currentJob.details) + "</p>";
        }
      } else {
        runnerHtml += "<p class='muted'>Idle.</p>";
      }
      runnerCard.innerHTML = runnerHtml;

      jobsBody.innerHTML = "";
      for (const job of snapshot.jobs) {
        const row = document.createElement("tr");
        row.innerHTML =
          "<td>" + escapeHtml(job.kind) + "</td>" +
          "<td>" + escapeHtml(job.target || "") + "</td>" +
          "<td class='" + statusClass(job.status) + "'>" + escapeHtml(job.status) + "</td>" +
          "<td>" + escapeHtml(composeTimes(job)) + "</td>" +
          "<td>" + escapeHtml(job.error || job.details || "") + "</td>" +
          "<td>" + escapeHtml(String(job.spawned || 0)) + "</td>" +
          "<td>" + (job.thumbnailUrl ? "<img class='thumb' src='" + escapeAttr(job.thumbnailUrl) + "' alt='thumbnail'>" : "") + "</td>";
        jobsBody.appendChild(row);
      }
      if (snapshot.jobs.length === 0) {
        const row = document.createElement("tr");
        row.innerHTML = "<td colspan='7' class='muted'>No jobs yet.</td>";
        jobsBody.appendChild(row);
      }
    }

    function composeTimes(job) {
      return [job.queuedAt, job.startedAt, job.finishedAt].filter(Boolean).join(" -> ");
    }

    function escapeHtml(value) {
      return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll("\"", "&quot;")
        .replaceAll("'", "&#39;");
    }

    function escapeAttr(value) {
      return escapeHtml(value);
    }

    render(initialSnapshot);

    const events = new EventSource("/events");
    events.onmessage = event => {
      try {
        render(JSON.parse(event.data));
      } catch (err) {
        console.error("bad event payload", err);
      }
    };
    events.onerror = err => {
      logError(err);
    };

    function logError(err) {
      console.error("event stream error", err);
    }
  </script>
</body>
</html>`
