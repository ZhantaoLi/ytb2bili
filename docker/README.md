# ytb2bili Docker 部署

本目录包含 Docker Compose 部署文件。当前 Compose 文件在 `docker/docker-compose.yml`，请从 `docker/` 目录启动，或在仓库根目录显式指定 `-f docker/docker-compose.yml`。

默认流程不需要付费 API。DeepLX、OpenAI 兼容接口、DeepSeek、Gemini 等都需要你自行配置端点和 Key，未配置时不会启用。

## 快速启动

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# 首次启动前，按下方“当前配置写法”修正 config.toml。
docker compose up -d
docker compose logs -f ytb2bili
```

打开 `http://localhost:8096`。

## 必要环境变量

第一次后台登录前必须显式配置管理员账号。可写入 `docker-compose.yml` 的 `ytb2bili.environment`：

```yaml
environment:
  - TZ=Asia/Shanghai
  - YTB2BILI_ADMIN_USERNAME=owner
  - YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
  - YTB2BILI_ADMIN_EMAIL=owner@example.local
  - YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

`YTB2BILI_ACCOUNT_ENCRYPTION_KEY` 建议固定配置为 16、24、32 字节，或 base64 解码后为这些长度。否则账号加密会使用进程级临时密钥，容器重启后可能无法稳定解密历史账号数据。

## 推荐挂载

当前 Compose 已挂载配置、日志和下载目录。建议额外挂载 `data`，用于持久化 cookies 等运行数据：

```yaml
volumes:
  - ./config.toml:/app/config.toml
  - ./logs:/app/logs
  - ./downloads:/app/downloads
  - ./data:/app/data
```

## 当前配置写法

当前程序读取的是顶层字段和命名 TOML 表。请按下面的字段修正 `config.toml`：

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

[ProxyConfig]
use_proxy = false
proxy_host = "http://host.docker.internal:10809"

[WhisperConfig]
enabled = true
language = "en"
model_path = ""
threads = 0
```

关键点：

- `fileUpDir` 是任务文件根目录，Docker 推荐 `/app/downloads`。
- `data_path` 是 cookies 等运行数据目录，Docker 推荐 `/app/data`。
- `auto_upload = false` 会让任务停在准备完成状态，适合先手动确认再上传。
- `yt_dlp_path` 可留空，程序会查找常见路径和 `PATH`。
- 代理使用 `[ProxyConfig].use_proxy` 和 `[ProxyConfig].proxy_host`。
- `[WhisperConfig].enabled = true` 选择 B站必剪字幕分支；Mimo ASR 不再使用。

## 可选 API 配置

字幕翻译默认免费：Bing 优先，Google 回退。

如需覆盖免费翻译，可自行配置：

```toml
[DeepLXConfig]
enabled = true
endpoint = "https://your-deeplx.example/translate"
source_lang = "EN"
target_lang = "ZH"
timeout = 30

[OpenAICompatibleConfig]
enabled = true
provider = "local"
api_key = "your-key"
base_url = "http://host.docker.internal:11434/v1"
model = "qwen2.5:7b"
timeout = 60
max_tokens = 4000
temperature = 0.7
```

如需 Gemini 生成元数据：

```toml
[GeminiConfig]
enabled = true
api_key = "your-key"
model = "gemini-2.5-flash"
timeout = 120
max_tokens = 8000
use_for_metadata = true
analyze_video = true
video_sample_frames = 0
```

## Bilibili 投稿配置

```toml
[BilibiliConfig]
copyright = 2
source = ""
no_reprint = 1
use_original_title = true
use_original_desc = true
custom_title_template = "{original_title}"
custom_desc_template = "{original_desc}"
tid = 122
dynamic = "发布了新视频！"
open_elec = 0
selection_reserve = 0
up_selection_reply = 0
up_close_reply = 0
up_close_reward = 1
```

标题和简介来源逻辑：

- `use_original_title = true` 优先使用原视频标题；否则优先使用生成标题。
- `use_original_desc = true` 优先使用原视频简介；否则优先使用生成简介。
- `custom_title_template` 支持 `{original_title}` 和 `{ai_title}`。
- `custom_desc_template` 支持 `{original_desc}` 和 `{ai_desc}`。

## 常用命令

```bash
docker compose ps
docker compose logs -f ytb2bili
docker compose restart ytb2bili
docker compose pull
docker compose up -d
docker compose down
```

清空数据库和本地容器数据属于破坏性操作，请确认后再执行：

```bash
docker compose down -v
```

## 排障

- 后台无法登录：检查 `YTB2BILI_ADMIN_USERNAME` 和 `YTB2BILI_ADMIN_PASSWORD` 是否已设置。
- YouTube 提示不是机器人验证：在 UI 上传 cookies，或把 `cookies.txt` 放到 `config.toml` 同目录。
- 下载走不了代理：确认 `[ProxyConfig].use_proxy = true`，Docker 访问宿主机代理通常使用 `http://host.docker.internal:端口`。
- 任务完成但未上传：这是 `auto_upload = false` 的预期行为，请在 Dashboard 手动上传，或确认后开启自动上传。
- 找不到视频文件：检查 `fileUpDir` 与 Docker 挂载路径是否一致。

## 相关链接

- Docker Hub: https://hub.docker.com/r/zhantaoli/ytb2bili
- Source: https://github.com/ZhantaoLi/ytb2bili
- Releases: https://github.com/ZhantaoLi/ytb2bili/releases
