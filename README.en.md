# ytb2bili

[简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

`ytb2bili` is a local workflow service for preparing YouTube videos and publishing them to Bilibili. It uses a Go backend, a Next.js admin UI, scheduled task chains, yt-dlp downloads, subtitle generation, free subtitle translation, optional AI metadata generation, and Bilibili upload.

The default workflow does not require paid APIs. All external model or translation APIs are disabled until you configure your own endpoint and key.

## Quick Start

Docker:

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# Edit config.toml with the current fields shown below before first start.
docker compose up -d
docker compose logs -f ytb2bili
```

Local backend:

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
cp config.toml.example config.toml
# Edit config.toml with the current fields shown below before first start.
go mod download
go run main.go
```

Frontend dev server:

```bash
cd web
npm install
npm run dev
```

Open the integrated app at `http://localhost:8096`, or the frontend dev server at `http://localhost:3000`.

Before first admin login, configure:

```bash
YTB2BILI_ADMIN_USERNAME=owner
YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
YTB2BILI_ADMIN_EMAIL=owner@example.local
YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

## Runtime Notes

- `CONFIG_FILE` selects the config file. If unset, `config.toml` is used.
- `go run main.go` starts migrations, the preparation scheduler, and the upload scheduler.
- `auto_upload = false` is the recommended local default. Prepared videos stop at status `200` for manual upload.
- Bilibili upload requires QR login and valid account cookies.
- YouTube downloads may require proxy and cookies.

## Minimal Config

```toml
listen = ":8096"
environment = "development"
debug = true
fileUpDir = "./data"
data_path = "./data"
yt_dlp_path = ""
auto_upload = false

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

Docker MySQL config uses the current field names:

```toml
[database]
type = "mysql"
host = "mysql"
port = 3306
username = "ytb2bili"
password = "ytb2bili@123"
database = "bili_up"
timezone = "Asia/Shanghai"
```

## Configurable Options

- `listen`, `environment`, `debug`, `fileUpDir`, `data_path`, `yt_dlp_path`, `auto_upload`, `primary_ai_service`.
- `[database]`: `type`, `host`, `port`, `username`, `password`, `database`, `ssl_mode`, `timezone`.
- `[auth]` and `[api_auth]`: admin JWT/session settings and optional signed API auth.
- `[ProxyConfig]`: proxy switch and proxy URL.
- `[WhisperConfig]`: enables the Bcut subtitle branch; Mimo ASR is no longer used.
- `[DeepLXConfig]`, `[OpenAICompatibleConfig]`, `[DeepSeekTransConfig]`, `[GeminiConfig]`: optional self-configured translation or metadata providers.
- `[BilibiliConfig]`: title/description source, templates, copyright/source, partition, dynamic text, and upload flags.
- `[TenCosConfig]`, `[AnalyticsConfig]`, `[TranslatorConfig]`, `[BaiduTransConfig]`: advanced optional integrations.

## Free Provider Flow

Subtitle translation tries configured providers first, then falls back to free Bing and Google translation. Metadata generation tries configured Gemini/OpenAI-compatible/DeepSeek providers, then falls back to original video metadata or the video ID.

For subtitles, enable `[WhisperConfig].enabled = true` to use the Bcut branch. It tries YouTube captions first, then existing subtitles, then Bcut ASR.

## Build And Check

```bash
make build
make build-web
make build-api
make build-prod
make quick-build
make test
```

Validation commands:

```bash
go test -timeout=60s ./...
cd web && npx tsc --noEmit
cd web && npm run lint
cd web && npm run build:prod
git diff --check
```

## Layout

- `main.go`: application entrypoint and scheduler startup.
- `internal/chain_task/`: task-chain handlers.
- `internal/handler/`: HTTP API handlers.
- `internal/core/types/app_config.go`: authoritative config structure.
- `web/`: Next.js admin UI.
- `docker/`: Docker Compose deployment files.

## License

[MIT License](LICENSE)
