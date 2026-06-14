# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

`ytb2bili` is a workflow system for local video translation playback and YouTube-to-Bilibili publishing. It includes a web admin panel, task-chain orchestration, subtitle processing, AI copy generation, subtitle audio synthesis, synchronized A/V preview, Bilibili upload, and subtitle upload.

Video tutorial: https://www.bilibili.com/video/BV1tCRTBBEJo

## Key Features

- Local video translation playback with translated subtitles and dubbed audio
- End-to-end YouTube to Bilibili workflow: download, audio extraction, transcription, translation, metadata generation, and upload
- Configurable task chain with step-level enable or disable control
- AI integrations for translation, title, description, tag generation, and dubbed audio synthesis
- Bilibili integration with QR-code login, video submission, subtitle upload, and account management
- Modern web admin built with Go and Next.js
  - Dark mode support with persistent theme preference
  - Mobile-responsive design for all screen sizes
  - Auto-upload toggle for flexible upload workflow
  - Task history management with deletion capability

## 5-Minute Docker Deployment

Docker Compose is the fastest way to get started. By default, it launches two services:

- `mysql`: stores tasks, accounts, and runtime data
- `ytb2bili`: web admin panel and processing service

### 1. Prerequisites

- Docker
- Docker Compose

### 2. Get Deployment Files

```bash
git clone https://github.com/difyz9/ytb2bili-docker.git
cd ytb2bili-docker
docker compose up -d
```

### 3. Minimal Configuration

The default `[database]` section already matches `docker-compose.yml`. Keep at least the following:

```toml
[server]
host = "0.0.0.0"
port = 8096
static_dir = "./downloads"
static_path = "/static"

[database]
type = "mysql"
host = "mysql"
port = 3306
user = "ytb2bili"
password = "ytb2bili@123"
dbname = "bili_up"
sslmode = ""
timezone = "Asia/Shanghai"
auto_migrate = true
table_prefix = ""

[workflow]
download_dir = "./downloads"
ytdlp_path = "/usr/local/bin/yt-dlp"
ffmpeg_path = "/usr/bin/ffmpeg"

# Add this if your network requires a proxy for YouTube:
# proxy_url = "http://127.0.0.1:7890"
```

### 4. Start the Services

```bash
docker compose up -d
docker compose logs -f
```

Then open `http://localhost:8096`

### 5. Basic Usage

1. Open the admin panel
2. Sign in with the Bilibili app QR code
3. Create a task and paste the video link
4. Wait for download, processing, and upload to finish

Useful commands:

```bash
docker compose ps
docker compose restart
docker compose down
```

For Docker-specific details, see [docker/README.md](docker/README.md).

## Architecture

The project has three main parts:

- Processing engine: Go backend for download, transcription, translation, metadata generation, audio synthesis, Bilibili upload, and subtitle upload
- Admin panel: Next.js frontend for task management, configuration, account management, manual uploads, and synchronized translation playback review
- Runtime support: `config.toml`, database, Docker assets, and project documentation

## Repository Structure

- `internal/`: backend application code, task chain, processing logic, and service wiring
- `pkg/`: reusable packages, Bilibili integration, utilities, and models
- `web/`: frontend web admin panel
- `docker/`: Docker build and deployment files
- `bin/`: example scripts, test helpers, and workflow files

## Local Development

### 1. Clone the Repository

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. Prepare Configuration

```bash
cp config.toml.example config.toml
```

Fill in database settings, download directory, proxy settings, and other required options. Optional API integrations are self-configured; the default free workflow does not require API keys. Start with [config.toml.example](config.toml.example).

### 3. Start the Backend

```bash
go mod download
go run main.go
```

### 4. Start the Frontend

```bash
cd web
npm install
npm run dev
```

### 5. Workflow

1. Open the web admin panel
2. Log in to a Bilibili account
3. Upload a local video to generate translated subtitles and dubbed audio, then review them in synchronized playback
4. Install the Chrome extension: https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest
5. Open any YouTube video page and submit the link through the extension
6. Check task status and logs in the admin panel
7. Rerun steps, edit copy, or manually submit to Bilibili when needed

## Processing Flow

1. Import a local video or submit a YouTube link
2. Download the video and thumbnail
3. Extract audio
4. Generate subtitles
5. Translate subtitles
6. Synthesize dubbed audio and preview with synchronized playback
7. Generate title, description, and tags
8. Upload the video to Bilibili
9. Upload subtitles to Bilibili

## Configuration and Build

The main runtime configuration file is `config.toml`:

```bash
cp config.toml.example config.toml
```

Common options include:

- `server.port`: service port
- `database.*`: database connection settings
- `workflow.*`: download directory, proxy, ffmpeg, yt-dlp, free subtitle generation, optional local translation, and upload pipeline
- `DeepLXConfig`: optional DeepLX subtitle translation; disabled by default, configure `endpoint` yourself
- `OpenAICompatibleConfig`: optional OpenAI-compatible subtitle translation; disabled by default, configure `base_url`, `model`, and `api_key` yourself
- `updater.enabled`: auto-update switch

Build commands:

```bash
make build
make build-dev
make build-linux-arm64
make build-all
```

If you need to extend the pipeline, start with the implementations under `internal/chain_task/`.

## Validation Commands

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## License and Contact

- License: [MIT License](LICENSE)
- GitHub: [@difyz9](https://github.com/difyz9)
- Project: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
- QQ group: 773066052

