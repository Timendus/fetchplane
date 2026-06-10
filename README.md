# Fetchplane

Small Floatplane downloader for personal use in a Docker container. It allows
you to log in to Floatplane and then scans subscribed creators on a schedule,
and stores videos these creators release. It can also remove old files if you so
desire.

## What it does

- Authenticates with Floatplane using OAuth device login
- Persists OAuth session tokens so later runs do not need a fresh device login
- Polls your subscriptions for creator posts with video attachments
- Downloads videos into per-creator and per-subchannel folders
- Saves thumbnails, `.nfo` files and subtitles next to downloaded videos
- Saves logos and cover art for the creators/channels

## Configuration

The app uses two paths:

- `LIBRARY_PATH`: where downloaded media is stored. Defaults to `/downloads`.
- `CONFIG_PATH`: where `config.yaml` and `session.json` live. Defaults to `/config`.

At startup it ensures these paths exist:

- `LIBRARY_PATH/`
- `CONFIG_PATH/session.json`
- `CONFIG_PATH/config.yaml`

If `config.yaml` does not exist yet, the app creates it with defaults and
comments. You then edit that file to set at least your Floatplane OAuth client
ID.

`config.yaml` holds:

- `oauthClientId`: required
- `oauthScope`: defaults to `openid videos:watch account:read`
- `pollInterval`: defaults to `600`
- `maxDaysToDownload`: defaults to `7`
- `removeOlderThanDays`: optional; `0` means keep indefinitely

## Docker Usage

Build the image:

```sh
make docker-build
```

Run it:

```sh
docker run --rm -it \
  -v "$PWD/downloads:/downloads" \
  -v "$PWD/config:/config" \
  fetchplane:latest
```

On first run, the container will create `/config/config.yaml`,
`/config/session.json`, and `/downloads/`. Edit `/config/config.yaml` and set
`oauthClientId`, then start the container again.

The image defaults to daemon mode. On first authenticated run it will print a
Floatplane device-login URL. Open the URL, complete login, and the container
will continue on its own.

For a one-shot run instead of daemon mode:

```sh
docker run --rm -it \
  -v "$PWD/downloads:/downloads" \
  -v "$PWD/config:/config" \
  --entrypoint /usr/local/bin/fetchplane \
  fetchplane:latest
```

## Local Usage

```sh
LIBRARY_PATH=storage/downloads CONFIG_PATH=storage/config go run ./cmd/fetchplane
```

Add `-daemon` to keep polling. Add `-debug` to see what requests are being done.

## Rough Internals

The current implementation is intentionally simple:

1. Start up, bootstrap the library/config paths, load `config.yaml`, and load
   the local OAuth session file.
2. Reuse a saved OAuth access token if still fresh.
3. Otherwise refresh with the saved refresh token.
4. Otherwise start a new OAuth device-login flow and persist the new tokens
   immediately.
5. Call `GET /api/v3/user/subscriptions` to enumerate subscribed creators.
6. For each creator, resolve creator metadata through `GET
/api/v3/creator/info`.
7. Prepare creator and channel folders plus artwork before scanning individual
   posts.
8. Page through `GET /api/v3/content/creator` and extract video attachment IDs
   from posts.
9. Treat the video file on disk as the source of truth: missing videos may be
   downloaded if they are within the configured download window existing videos
   have their sidecars repaired if needed
10. Optionally remove managed videos older than the configured retention window.

The session file only tracks OAuth tokens and expiry. Downloaded videos on disk
are the canonical sync state.

## Notes For Developers

- The live Floatplane API does not fully match older unofficial docs. Favor
  working behavior over doc assumptions.
- OAuth is device-code based now; username/password login is intentionally not
  used.
- Response shapes can vary. In particular, creator subscriptions and content
  attachments have already drifted from older expectations.
- The codebase is small on purpose. Prefer straightforward changes over
  abstractions unless the live API forces them.
