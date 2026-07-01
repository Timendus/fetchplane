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

  let authHtml = "<p><strong>Session status:</strong> ";
  authHtml += snapshot.authenticated
    ? "<span class='status-ok'>authenticated</span>"
    : "<span class='status-warn'>login required</span>";
  authHtml += "</p>";
  authHtml += "<dl class='job-times muted'>";
  if (snapshot.tokenExpiresAt) {
    authHtml +=
      "<div><dt>Token expiry</dt><dd>" +
      escapeHtml(snapshot.tokenExpiresAt) +
      "</dd></div>";
  }
  if (snapshot.pendingAuth) {
    authHtml +=
      "<div><dt>Login URL</dt><dd><a href='" +
      escapeAttr(snapshot.pendingAuth.url) +
      "' target='_blank'>" +
      escapeHtml(snapshot.pendingAuth.url) +
      "</a></dd></div>";
    authHtml +=
      "<div><dt>Code</dt><dd><code>" +
      escapeHtml(snapshot.pendingAuth.userCode) +
      "</code></dd></div>";
    authHtml +=
      "<div><dt>Login expiry</dt><dd>" +
      escapeHtml(snapshot.pendingAuth.expiresAt) +
      "</dd></div>";
  }
  authHtml += "</dl>";
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
      "<p><strong>Current job:</strong> " +
      escapeHtml(currentJob.kind) +
      " " +
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
    const item = document.createElement("li");
    item.className = "job-item";
    const card = document.createElement("article");
    card.className = "job is-" + escapeCss(job.status);

    const detailText = job.error || job.details || "No extra details.";
    const times = composeTimes(job)
      .map((line) => {
        const parts = line.split(" ");
        return "<div><dt>" + escapeHtml(parts.shift()) + "</dt><dd>" + escapeHtml(parts.join(" ")) + "</dd></div>";
      })
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
      "<figure class='job-media'>" +
      media +
      "</figure>" +
      "<section class='job-body'>" +
      "<header class='job-header'>" +
      "<div class='job-title'>" +
      "<p class='job-kind'>" +
      escapeHtml(job.kind) +
      "</p>" +
      "<h3 class='job-target'>" +
      escapeHtml(job.target || job.kind) +
      "</h3>" +
      "</div>" +
      "<p class='job-status " +
      statusClass(job.status) +
      "'>" +
      escapeHtml(job.status) +
      "</p>" +
      "</header>" +
      "<p class='job-details" +
      (job.error ? " job-error" : "") +
      "'>" +
      escapeHtml(detailText) +
      "</p>" +
      "<footer class='job-footer'>" +
      "<span></span>" +
      "<dl class='job-times muted'>" +
      times +
      "</dl>" +
      "</footer>" +
      "</section>";
    item.appendChild(card);
    jobsList.appendChild(item);
  }
  if (snapshot.jobs.length === 0) {
    const empty = document.createElement("li");
    empty.className = "job-item muted";
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
