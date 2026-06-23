# ytb2bili

[English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

`ytb2bili` 是一个本地运行的 YouTube 到 Bilibili 处理系统。当前项目由 Go 后端、Next.js 管理后台、任务链调度器、yt-dlp 下载、字幕生成、免费字幕翻译、可选 AI 元数据生成和 B 站投稿链路组成。

默认流程不需要付费 API。外部大模型、翻译或元数据 API 都是可选项，只有在你自行配置端点和 Key 后才会启用。

## 当前能力

- 通过 `yt-dlp` 下载视频，支持代理、YouTube cookies 和 Chrome cookies 回退。
- 通过 YouTube 字幕或 B站必剪分支生成字幕。
- 默认用免费 Bing 翻译字幕，失败后回退 Google。
- 可选接入 DeepLX、DeepSeek、OpenAI 兼容 API 或 Gemini。
- 生成投稿元数据并写入 `meta.json`。
- B 站扫码登录后上传视频和字幕。
- Dashboard 支持任务历史记录和硬删除任务记录。
- 默认关闭自动上传，本地运行时任务会停在准备就绪状态，方便手动确认。

## Docker 快速启动

Compose 文件在 `docker/` 目录，不在仓库根目录。

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# 首次启动前，按下方“当前最小配置”修正 config.toml。
docker compose up -d
docker compose logs -f ytb2bili
```

打开 `http://localhost:8096`。

第一次登录后台前，需要显式配置管理员账号。可加入 `docker-compose.yml` 的 `ytb2bili.environment`，也可以在启动服务的 shell 中导出：

```bash
YTB2BILI_ADMIN_USERNAME=owner
YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
YTB2BILI_ADMIN_EMAIL=owner@example.local
YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

`YTB2BILI_ACCOUNT_ENCRYPTION_KEY` 必须是 16、24、32 字节，或 base64 解码后是这些长度。未配置时会生成进程级临时密钥，B 站账号加密数据可能无法跨重启稳定解密。

## 本地开发

后端：

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
cp config.toml.example config.toml
# 首次启动前，按下方“当前最小配置”修正 config.toml。
go mod download
go run main.go
```

`go run main.go` 会启动 HTTP 服务、数据库迁移、准备阶段调度器和上传调度器。建议本地测试保持 `auto_upload = false`，除非你明确要让就绪视频自动投稿。

前端：

```bash
cd web
npm install
npm run dev
```

后端默认是 `http://localhost:8096`，前端开发服务默认是 `http://localhost:3000`。前端 API 默认指向 `http://localhost:8096/api/v1`，所以需要保持 Go 后端运行。

## 构建命令

当前 Makefile 中真实存在的常用目标：

- `make build`：构建前端静态资源并打包 Go 二进制。
- `make build-web`：只构建前端，并复制到 `internal/web/bili-up-web`。
- `make build-api`：只构建 Go 后端。
- `make build-prod`：生产构建，开启更小体积的编译参数。
- `make quick-build`：只快速构建 Go，不刷新前端。
- `make test`：运行 Go 测试。
- `make dev`：如果已安装 Air，则以开发模式运行。
- `make lint`、`make fmt`、`make clean`、`make info`：质量检查和维护命令。

## 当前最小配置

程序从 `CONFIG_FILE` 指定的文件读取配置；未指定时读取 `config.toml`。当前代码使用顶层字段和命名 TOML 表，历史的 server/workflow 风格字段不是当前运行入口。

本地 SQLite 示例：

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

Docker MySQL 示例：

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

Docker 环境如需持久化 cookies、账号相关运行数据，建议额外挂载 `./data:/app/data`。

## 可配置项

- `listen`：后端监听地址，例如 `:8096`。
- `environment`：运行环境标识，例如 `development` 或 `production`。
- `debug`：控制后端日志详细程度。
- `fileUpDir`：视频、音频、字幕、封面等任务文件根目录。
- `data_path`：运行数据目录，包含上传的 YouTube cookies 等。
- `yt_dlp_path`：可选的 yt-dlp 安装目录。留空会查找常见路径和 `PATH`。
- `auto_upload`：为 `false` 时任务停在 `200` 就绪态，需要手动上传；为 `true` 时上传调度器会自动投稿。
- `primary_ai_service`：可选 AI 服务选择器，留空则走自动回退逻辑。
- `[database]`：数据库连接。支持 `sqlite`、`mysql`、`postgres`、`postgresql`，字段包括 `type`、`host`、`port`、`username`、`password`、`database`、`ssl_mode`、`timezone`。
- `[auth]`：后台 JWT 和 session 配置。
- `[api_auth]`：可选的签名 API 认证和 cookies 解密配置。
- `[ProxyConfig]`：代理开关和代理地址，供 yt-dlp 和部分 HTTP 客户端使用。
- `[WhisperConfig]`：历史命名仍保留，但 `enabled = true` 实际选择 B站必剪字幕分支；Mimo ASR 不再使用。
- `[DeepLXConfig]`：可选字幕翻译，需自行配置完整 `/translate` 端点。
- `[OpenAICompatibleConfig]`：可选 OpenAI 兼容接口，可用于字幕翻译和元数据生成，需自行配置 `base_url`、`model`、`api_key`。
- `[DeepSeekTransConfig]`：可选 DeepSeek 翻译和元数据生成配置。
- `[GeminiConfig]`：可选 Gemini 元数据生成，需设置 `enabled = true` 和 `use_for_metadata = true`。
- `[BilibiliConfig]`：B 站投稿元信息配置，包括版权、来源、标题/简介来源、自定义模板、分区、动态文本、评论和打赏开关。
- `[TenCosConfig]`：可选腾讯云 COS 存储配置。
- `[AnalyticsConfig]`：可选数据分析客户端配置。
- `[TranslatorConfig]`、`[BaiduTransConfig]`：高级翻译框架配置，默认任务链不依赖它们。

## 免费与可选 API 流程

字幕翻译选择顺序：

1. 已启用且配置完整的 DeepLX。
2. 已启用且配置完整的 OpenAI 兼容接口。
3. 已启用且配置完整的 DeepSeek。
4. 免费 Bing 翻译。
5. 免费 Google 回退。

元数据生成选择顺序：

1. 已启用并用于元数据的 Gemini。
2. 已启用的 OpenAI 兼容接口。
3. 已启用的 DeepSeek。
4. 免费回退：使用原视频标题/简介，缺失时使用视频 ID。

字幕生成分支：

1. `[WhisperConfig].enabled = true` 时，使用 B站必剪分支。该分支会先尝试 YouTube 字幕，再复用本地字幕，最后调用 B站必剪 ASR。
2. 未启用该分支时，旧的 `GenerateSubtitles` 步骤依赖数据库中已有字幕 JSON。

## 使用流程

1. 准备 `config.toml`，至少配置 `fileUpDir`、数据库、可选代理和可选字幕分支。
2. 启动后端；如需改前端，另外启动 `web` 开发服务。
3. 用环境变量中配置的管理员账号登录。
4. 扫码绑定 B 站账号。
5. 提交 YouTube URL 或视频 ID。
6. 在 Dashboard 查看任务步骤、重试失败步骤或删除历史任务。
7. 本地建议手动上传；确认稳定后再考虑开启 `auto_upload`。

YouTube 要求登录验证时，可在 UI 上传 cookies，或把 `cookies.txt` 放到 `config.toml` 同目录。下载器还会尝试读取 Chrome cookies 作为回退。

## 验证命令

```bash
go test -timeout=60s ./...
cd web && npx tsc --noEmit
cd web && npm run lint
cd web && npm run build:prod
git diff --check
```

纯文档改动通常只需要文本检查和 `git diff --check`；涉及代码时再跑完整验证。

## 项目结构

- `main.go`：应用入口，负责依赖装配、调度器、路由和嵌入式前端服务。
- `internal/chain_task/`：准备阶段和上传阶段任务链。
- `internal/handler/`：HTTP API 与后台路由。
- `internal/core/types/app_config.go`：当前运行配置结构的权威来源。
- `pkg/`：B 站账号、认证、字幕、翻译、工具等可复用模块。
- `web/`：Next.js 管理后台。
- `internal/web/bili-up-web`：嵌入到 Go 服务中的前端静态产物。
- `docker/`：Docker Compose 和容器部署文件。

## 许可证

[MIT License](LICENSE)
