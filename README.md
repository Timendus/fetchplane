# Fetchplane

Small Floatplane downloader for personal use in a Docker container. It logs in through Floatplane's OAuth device flow, scans subscribed creators on a schedule, and stores video files plus a small local session file.

## What It Does

- Authenticates with Floatplane using OAuth device login
- Polls your subscriptions for creator posts with video attachments
- Downloads videos into per-creator and per-subchannel folders
- Saves thumbnails and `.nfo` files next to downloaded videos
- Persists OAuth session tokens so later runs do not need a fresh device login

## Configuration

The program reads environment variables directly.

Use [.env.example](/home/tim/Workspace/fetchplane/.env.example) as a starting point:

- `FP_OAUTH_CLIENT_ID`: required; must be a Floatplane-issued OAuth client ID
- `FP_OAUTH_SCOPE`: optional; defaults to `openid videos:watch account:read`
- `FP_DOWNLOAD_DIR`: optional; defaults to `downloads`
- `FP_SESSION_FILE`: optional; defaults to `$FP_DOWNLOAD_DIR/.fetchplane-session.json`
- `FP_POLL_INTERVAL`: optional; seconds between runs in daemon mode; defaults to `300`
- `FP_MAX_DAYS_TO_DOWNLOAD`: optional; only download missing videos from the last N days; defaults to `7`
- `FP_REMOVE_OLDER_THAN_DAYS`: optional; delete managed videos older than N days

## Docker Usage

Build the image:

```sh
make docker-build
```

Run it:

```sh
docker run --rm -it \
  --env-file .env \
  -v "$PWD/downloads:/data" \
  fetchplane:latest
```

The image defaults to daemon mode. On first run it will print a Floatplane device-login URL and code. Open the URL, complete login, and the container will continue on its own.

For a one-shot run instead of daemon mode:

```sh
docker run --rm -it \
  --env-file .env \
  -v "$PWD/downloads:/data" \
  --entrypoint /usr/local/bin/fetchplane \
  fetchplane:latest
```

## Local Usage

```sh
export $(grep -v '^#' .env | xargs)
go run ./cmd/fetchplane
```

Add `-daemon` to keep polling:

```sh
go run ./cmd/fetchplane -daemon
```

## Rough Internals

The current implementation is intentionally simple:

1. Start up, load config, and load the local OAuth session file.
2. Reuse a saved OAuth access token if still fresh.
3. Otherwise refresh with the saved refresh token.
4. Otherwise start a new OAuth device-login flow and persist the new tokens immediately.
5. Call `GET /api/v3/user/subscriptions` to enumerate subscribed creators.
6. For each creator, resolve creator metadata through `GET /api/v3/creator/info`.
7. Prepare creator and channel folders plus artwork before scanning individual posts.
8. Page through `GET /api/v3/content/creator` and extract video attachment IDs from posts.
9. Treat the video file on disk as the source of truth:
   missing videos may be downloaded if they are within the configured download window
   existing videos have their sidecars repaired if needed
10. Optionally remove managed videos older than the configured retention window.

The session file only tracks OAuth tokens and expiry. Downloaded videos on disk are the canonical sync state.

## Notes For Developers

- The live Floatplane API does not fully match older unofficial docs. Favor working behavior over doc assumptions.
- OAuth is device-code based now; username/password login is intentionally not used.
- Response shapes can vary. In particular, creator subscriptions and content attachments have already drifted from older expectations.
- The codebase is small on purpose. Prefer straightforward changes over abstractions unless the live API forces them.
