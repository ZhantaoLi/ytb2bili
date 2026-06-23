# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

`ytb2bili` is a local workflow service for preparing YouTube videos and publishing them to Bilibili. The current stack is a Go backend with Gin/GORM/Fx, a Next.js admin UI, a task-chain scheduler, yt-dlp downloads, subtitle generation, free subtitle translation, optional AI metadata generation, and Bilibili upload.

The default flow is free to run. Optional API providers are disabled unless you configure your own endpoint and key.

## Current Features

- Download videos with `yt-dlp`, including proxy support and YouTube cookies fallback.
- Generate subtitles from existing YouTube captions or the Bcut ASR branch.
- Translate subtitles with free Bing first and Google fallback by default.
- Optionally use DeepLX, DeepSeek, OpenAI-compatible APIs, or Gemini when self-configured.
- Generate upload metadata and write `meta.json`.
- Upload videos and subtitles to Bilibili after QR login.
- Manage task history from the dashboard, including hard deletion of task records.
- Keep automatic upload disabled by default so local runs stop at the ready-to-upload state.

## Quick Start With Docker

The compose file is under `docker/`, not the repository root.

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# Edit config.toml with the current fields shown below before first start.
docker compose up -d
docker compose logs -f ytb2bili
```

Open `http://localhost:8096`.

Before the first admin login, add explicit admin credentials to the `ytb2bili` service environment or export them in the shell that starts the service:

```bash
YTB2BILI_ADMIN_USERNAME=owner
YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
YTB2BILI_ADMIN_EMAIL=owner@example.local
YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

Use a 16, 24, or 32 byte value for `YTB2BILI_ACCOUNT_ENCRYPTION_KEY`, or a base64 value that decodes to one of those lengths. Without it, encrypted Bilibili account rows use a process-local key and may not survive restarts.

## Local Development

### Backend

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
cp config.toml.example config.toml
# Edit config.toml with the current fields shown below before first start.
go mod download
go run main.go
```

`go run main.go` starts the HTTP server, database migrations, the preparation scheduler, and the upload scheduler. Keep `auto_upload = false` during local testing unless you intentionally want ready videos uploaded automatically.

The backend listens on `http://localhost:8096` by default.

### Frontend

```bash
cd web
npm install
npm run dev
```

The frontend dev server listens on `http://localhost:3000`. Its API client defaults to `http://localhost:8096/api/v1`, so keep the Go backend running.

For a production-style build:

```bash
make build
```

Useful targets that exist today:

- `make build`: build frontend assets and the Go binary.
- `make build-web`: build only the static frontend and copy it to `internal/web/bili-up-web`.
- `make build-api`: build only the Go backend.
- `make build-prod`: production build with smaller binary flags.
- `make quick-build`: build Go only without refreshing frontend assets.
- `make test`: run Go tests.
- `make dev`: run Air if installed.
- `make lint`, `make fmt`, `make clean`, `make info`: quality and maintenance helpers.

## Minimal Current Config

The runtime config is loaded from `CONFIG_FILE` or `config.toml`. The current Go config uses top-level fields plus named TOML tables. Historical server/workflow-style fields are not the runtime source of truth.

Local SQLite example:

```toml
listen = ":8096"
environment = "development"
debug = true
fileUpDir = "./data"
data_path = "./data"
yt_dlp_path = ""
auto_upload = false
primary_ai_service = ""

[database]
type = "sqlite"
database = "data/ytb2bili.db"
timezone = "Asia/Shanghai"

[auth]
jwt_secret = ""
jwt_expiration = 24
session_secret = "change-this-session-secret"

[api_auth]
app_id = ""
app_secret = ""
cookies_decrypt_key = ""

[ProxyConfig]
use_proxy = false
proxy_host = "http://127.0.0.1:10809"

[WhisperConfig]
enabled = true
language = "en"
model_path = ""
threads = 0
```

Docker MySQL example:

```toml
listen = ":8096"
environment = "production"
debug = false
fileUpDir = "/app/downloads"
data_path = "/app/data"
yt_dlp_path = ""
auto_upload = false

[database]
type = "mysql"
host = "mysql"
port = 3306
username = "ytb2bili"
password = "ytb2bili@123"
database = "bili_up"
timezone = "Asia/Shanghai"
```

If you use Docker and want cookies/account data to persist outside the database, mount a data directory such as `./data:/app/data`.

## Configurable Options

- `listen`: backend listen address, for example `:8096`.
- `environment`: runtime label, usually `development` or `production`.
- `debug`: enables less-silent backend logging.
- `fileUpDir`: local media workspace. Task directories are created below this path.
- `data_path`: runtime data path, including uploaded YouTube cookie files.
- `yt_dlp_path`: optional yt-dlp install directory. Leave empty to use common paths and `PATH`.
- `auto_upload`: when `false`, prepared videos stop at status `200` for manual upload; when `true`, the upload scheduler can publish ready videos.
- `primary_ai_service`: optional selector for AI-related flows; empty means automatic fallback logic.
- `[database]`: `type`, `host`, `port`, `username`, `password`, `database`, `ssl_mode`, `timezone`. Supported types are `sqlite`, `mysql`, `postgres`, and `postgresql`.
- `[auth]`: JWT and session settings for admin authentication.
- `[api_auth]`: optional signed API credentials and cookie decrypt key.
- `[ProxyConfig]`: `use_proxy` and `proxy_host`; used by yt-dlp and selected HTTP clients.
- `[WhisperConfig]`: despite the historical name, `enabled = true` selects the Bcut subtitle branch. `language` controls the ASR language hint. Mimo ASR is no longer used.
- `[DeepLXConfig]`: optional subtitle translation provider. Configure your own full `/translate` endpoint.
- `[OpenAICompatibleConfig]`: optional OpenAI-compatible chat API for subtitle translation and metadata generation. Configure your own `base_url`, `model`, and `api_key`.
- `[DeepSeekTransConfig]`: optional DeepSeek-compatible translation and metadata provider.
- `[GeminiConfig]`: optional Gemini metadata provider. Set `enabled = true` and `use_for_metadata = true` to use it for metadata.
- `[BilibiliConfig]`: upload metadata choices, including copyright/source, title and description source, custom templates, partition `tid`, dynamic text, and reply/reward switches.
- `[TenCosConfig]`: optional Tencent COS storage settings.
- `[AnalyticsConfig]`: optional analytics client settings.
- `[TranslatorConfig]`, `[BaiduTransConfig]`: advanced translator framework settings; they are not required for the default task chain.

## Free And Optional Provider Flow

Subtitle translation provider order:

1. DeepLX if enabled and configured.
2. OpenAI-compatible API if enabled and complete.
3. DeepSeek if enabled and configured.
4. Free Bing translation.
5. Free Google fallback.

Metadata provider order:

1. Gemini if enabled and configured for metadata.
2. OpenAI-compatible API if enabled.
3. DeepSeek if enabled.
4. Free fallback from original title/description or the video ID.

Subtitle generation:

1. With `[WhisperConfig].enabled = true`, the Bcut branch first tries YouTube captions, then existing local subtitles, then Bcut ASR.
2. With that branch disabled, the legacy `GenerateSubtitles` step expects subtitle JSON already stored in the database.

## Usage Flow

1. Prepare `config.toml` with a real `fileUpDir`, database config, optional proxy, and optional ASR/provider settings.
2. Start the backend and, for UI development, start the frontend dev server.
3. Log in as the configured admin user.
4. Bind a Bilibili account by QR code.
5. Submit a YouTube URL or video ID.
6. Inspect task steps on the dashboard.
7. Manually upload the prepared video, or set `auto_upload = true` only when you intentionally want scheduled upload.

For YouTube bot checks, upload cookies from the UI or place `cookies.txt` beside `config.toml`. The downloader also tries Chrome cookies as a fallback on supported local environments.

## Validation Commands

```bash
go test -timeout=60s ./...
cd web && npx tsc --noEmit
cd web && npm run lint
cd web && npm run build:prod
git diff --check
```

Docs-only edits usually only need text checks and `git diff --check`, but use the full commands when code changes are involved.

## Repository Layout

- `main.go`: application entrypoint, dependency wiring, schedulers, routes, and embedded frontend serving.
- `internal/chain_task/`: preparation and upload task-chain logic.
- `internal/handler/`: HTTP handlers and admin/API routes.
- `internal/core/types/app_config.go`: authoritative runtime config structure.
- `pkg/`: reusable packages, Bilibili account storage, auth, subtitle, translator, and utility code.
- `web/`: Next.js admin frontend.
- `internal/web/bili-up-web`: embedded static frontend output.
- `docker/`: Docker Compose and container deployment files.

## License

[MIT License](LICENSE)
