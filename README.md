# Fetchplane

Fetchplane is a simple Floatplane downloader for personal use, intended
primarily as a Docker container. It allows you to log in to Floatplane and then
scans subscribed creators on a schedule, and stores videos these creators
release. It can also remove old files if you so desire.

## What it does

- Authenticates with Floatplane
- Polls subscriptions for new videos
- Downloads videos into per-creator and per-subchannel folders
- Saves thumbnails, `.nfo` files and subtitles next to downloaded videos
- Saves logos and cover art for the creators/channels
- Has a simple web interface to log in with and to show what's being downloaded

## Configuration

The app uses two paths in environment variables:

- `LIBRARY_PATH`: where downloaded media is stored. Defaults to `/downloads`.
- `CONFIG_PATH`: where `config.yaml` and `session.json` live. Defaults to `/config`.

At startup it ensures these paths exist, and creates them if needed:

- `LIBRARY_PATH/`
- `CONFIG_PATH/session.json`
- `CONFIG_PATH/config.yaml`

`session.json` is managed be the application, and stores information about your
session with Floatplane. Don't share this file with other people or they will
have access to your Floatplane account.

If `config.yaml` does not exist yet, the app creates it with defaults and
comments. `config.yaml` holds:

_To be hard-coded in the future:_

- `oauthClientId`: required for Floatplane to accept our application
- `oauthScope`: defaults to `openid videos:watch account:read`

_To be kept:_

- `listen`: where to run the web service, defaults to `:38080`
- `pollInterval`: how often to check for new videos, defaults to `600` seconds (10 minutes)
- `maxDaysToDownload`: how many days of backlog to download, mostly for first start, defaults to `7` days
- `removeOlderThanDays`: how old videos have to be before we delete them, defaults to `0` days, meaning never delete

## Authentication

When you run the application and it is not authenticated, it will output a
Floatplane login URL. Open the URL, complete login, and the application will be
authenticated to start downloading videos.

Alternatively, navigate to the web interface (`http://localhost:38080` by
default) and click "Log in" to start the same flow.

## Docker Usage

Requires:

- Docker
- make

Build the image:

```sh
git clone git@github.com:Timendus/fetchplane.git
cd fetchplane
make docker-build
```

Run it:

```sh
docker run --rm -it \
  -p 38080:38080 \
  -v "$PWD/downloads:/downloads" \
  -v "$PWD/config:/config" \
  fetchplane:latest
```

The container serves the web UI on port `38080` by default, so after starting
the container you can open `http://localhost:38080`.

If you want HTTP request/response logging, pass the `-debug` flag:

```sh
docker run --rm -it \
  -p 38080:38080 \
  -v "$PWD/downloads:/downloads" \
  -v "$PWD/config:/config" \
  fetchplane:latest \
  -debug
```

## Local Usage

Requires:

- Go
- make

To just run locally:

```sh
git clone git@github.com:Timendus/fetchplane.git
cd fetchplane
make run
```

To build a binary to run on another machine:

```sh
make build
ls -la dist
```

Add `-debug` as a flag to the program to see which requests are being fired.

## License

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU Affero General Public License as published by the Free
Software Foundation, either version 3 of the License, or (at your option) any
later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License along
with this program. If not, see <https://www.gnu.org/licenses/>.
