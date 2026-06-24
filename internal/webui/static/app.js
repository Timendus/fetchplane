function statusClass(status) {
  if (status === "succeeded") return "status-ok";
  if (status === "failed") return "status-bad";
  if (status === "skipped") return "status-warn";
  return "";
}

function composeTimes(job) {
  const times = [];
  if (job.queuedAt) times.push("Queued " + job.queuedAt);
  if (job.startedAt) times.push("Started " + job.startedAt);
  if (job.finishedAt) times.push("Finished " + job.finishedAt);
  return times;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeAttr(value) {
  return escapeHtml(value);
}

function escapeCss(value) {
  return String(value).replace(/[^a-z0-9_-]/gi, "");
}

function jobIcon(kind) {
  if (kind === "auth.login") {
    return `
      <div class="thumb-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <path d="M7 11V8a5 5 0 0 1 10 0v3"></path>
          <rect x="5" y="11" width="14" height="10" rx="2"></rect>
          <path d="M12 15v2"></path>
        </svg>
      </div>`;
  }
  if (kind === "sync.subscriptions") {
    return `
      <div class="thumb-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <path d="M4 7h16"></path>
          <path d="M4 12h10"></path>
          <path d="M4 17h8"></path>
          <path d="M17 11l3 3-3 3"></path>
        </svg>
      </div>`;
  }
  if (kind === "sync.creator") {
    return `
      <div class="thumb-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <circle cx="12" cy="8" r="3.5"></circle>
          <path d="M5 19a7 7 0 0 1 14 0"></path>
        </svg>
      </div>`;
  }
  if (kind === "prune.channel") {
    return `
      <div class="thumb-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <path d="M4 7h16"></path>
          <path d="M9 7V5h6v2"></path>
          <path d="M7 7l1 12h8l1-12"></path>
          <path d="M10 11v5"></path>
          <path d="M14 11v5"></path>
        </svg>
      </div>`;
  }
  return `
    <div class="thumb-icon" aria-hidden="true">
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="8"></circle>
        <path d="M12 8v4"></path>
        <path d="M12 16h.01"></path>
      </svg>
    </div>`;
}

function render(snapshot) {
  const authCard = document.getElementById("auth-card");
  const runnerCard = document.getElementById("runner-card");
  const jobsList = document.getElementById("jobs-list");
  const loginForm = document.getElementById("login-form");

  let authHtml = "<p>Status: ";
  authHtml += snapshot.authenticated
    ? "<span class='status-ok'>authenticated</span>"
    : "<span class='status-warn'>login required</span>";
  authHtml += "</p>";
  if (snapshot.tokenExpiresAt) {
    authHtml +=
      "<p class='muted'>Token expiry: " +
      escapeHtml(snapshot.tokenExpiresAt) +
      "</p>";
  }
  if (snapshot.pendingAuth) {
    authHtml +=
      "<p>Current login URL: <a href='" +
      escapeAttr(snapshot.pendingAuth.url) +
      "' target='_blank'>" +
      escapeHtml(snapshot.pendingAuth.url) +
      "</a></p>";
    authHtml +=
      "<p>Enter code <code>" +
      escapeHtml(snapshot.pendingAuth.userCode) +
      "</code></p>";
    authHtml +=
      "<p class='muted'>Expires at " +
      escapeHtml(snapshot.pendingAuth.expiresAt) +
      "</p>";
  }
  if (snapshot.lastAuthError) {
    authHtml +=
      "<p class='status-bad'>Last auth error: " +
      escapeHtml(snapshot.lastAuthError) +
      "</p>";
  }
  authCard.innerHTML = authHtml;
  loginForm.hidden = snapshot.authenticated;

  const currentJob =
    snapshot.jobs.find((job) => job.status === "running") ||
    snapshot.jobs.find((job) => job.status === "queued");
  let runnerHtml = "";
  if (currentJob) {
    runnerHtml +=
      "<p>Current job: <strong>" +
      escapeHtml(currentJob.kind) +
      "</strong> " +
      escapeHtml(currentJob.target || "") +
      "</p>";
    if (currentJob.details) {
      runnerHtml +=
        "<p class='muted'>" + escapeHtml(currentJob.details) + "</p>";
    }
  } else {
    runnerHtml += "<p class='muted'>Idle.</p>";
  }
  runnerCard.innerHTML = runnerHtml;

  jobsList.innerHTML = "";
  for (const job of snapshot.jobs) {
    const card = document.createElement("article");
    card.className = "job is-" + escapeCss(job.status);

    const detailText = job.error || job.details || "No extra details.";
    const times = composeTimes(job)
      .map((line) => "<div>" + escapeHtml(line) + "</div>")
      .join("");

    let media = "";
    if (job.thumbnailUrl) {
      media =
        "<img class='thumb' src='" +
        escapeAttr(job.thumbnailUrl) +
        "' alt='thumbnail'>";
    } else if (job.kind === "video.download") {
      media = "<div class='thumb-placeholder'>Pending thumb</div>";
    } else {
      media = jobIcon(job.kind);
    }

    card.innerHTML =
      "<div class='job-media'>" +
      media +
      "</div>" +
      "<div class='job-body'>" +
      "<div class='job-header'>" +
      "<div class='job-title'>" +
      "<div class='job-kind'>" +
      escapeHtml(job.kind) +
      "</div>" +
      "</div>" +
      "<div class='job-status " +
      statusClass(job.status) +
      "'>" +
      escapeHtml(job.status) +
      "</div>" +
      "</div>" +
      "<div class='job-details" +
      (job.error ? " job-error" : "") +
      "'>" +
      escapeHtml(detailText) +
      "</div>" +
      "<div class='job-footer'>" +
      "<div></div>" +
      "<div class='job-times muted'>" +
      times +
      "</div>" +
      "</div>" +
      "</div>";
    jobsList.appendChild(card);
  }
  if (snapshot.jobs.length === 0) {
    const empty = document.createElement("div");
    empty.className = "muted";
    empty.textContent = "No jobs yet.";
    jobsList.appendChild(empty);
  }
}

async function loadSnapshot() {
  const response = await fetch("/api/snapshot", {
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    throw new Error("snapshot request failed");
  }
  return response.json();
}

function connectEvents() {
  const events = new EventSource("/events");
  events.onmessage = (event) => {
    try {
      render(JSON.parse(event.data));
    } catch (err) {
      console.error("bad event payload", err);
    }
  };
  events.onerror = (err) => {
    console.error("event stream error", err);
  };
}

function bindActions() {
  const syncForm = document.getElementById("sync-form");
  syncForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      await fetch("/sync/now", { method: "POST" });
    } catch (err) {
      console.error("sync trigger failed", err);
    }
  });
}

async function main() {
  bindActions();
  try {
    render(await loadSnapshot());
  } catch (err) {
    console.error("initial snapshot failed", err);
  }
  connectEvents();
}

window.addEventListener("DOMContentLoaded", () => {
  void main();
});
