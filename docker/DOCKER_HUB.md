# difyz9/ytb2bili

> YouTube / TikTok → Bilibili 全自动搬运系统  
> 默认使用免费流程：下载、生成字幕、定时发布，开箱即用。

[![Docker Pulls](https://img.shields.io/docker/pulls/difyz9/ytb2bili)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Image Size](https://img.shields.io/docker/image-size/difyz9/ytb2bili/latest)](https://hub.docker.com/r/difyz9/ytb2bili)
[![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-blue)](https://hub.docker.com/r/difyz9/ytb2bili)

---

## 快速开始

### 1. 创建工作目录并下载配置文件

```bash
mkdir ytb2bili && cd ytb2bili

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/config.toml \
     -o config.toml

curl -fsSL https://raw.githubusercontent.com/difyz9/ytb2bili-docker/main/docker-compose.yml \
     -o docker-compose.yml
```

### 2. 启动服务

```bash
docker compose up -d
```

服务就绪后访问 **http://localhost:8096**，用 B 站 App 扫码登录即可开始搬运。

---

## 免费字幕流程

默认不需要任何云端 LLM Key。系统会生成并上传原始字幕。
如果你在本机运行 Ollama 等 OpenAI 兼容服务，可以手动开启本地翻译增强。
所有可选 API 或本地兼容 API 都需要使用者自行配置，项目不内置 Key，也不默认启用。

### 默认免费配置

```toml
[workflow]
llm_translation_enabled     = false
llm_translation_source_lang = "en"
llm_translation_target_lang = "zh-Hans"
llm_translation_batch_size  = 25    # 每批翻译字幕条数
llm_translation_max_workers = 3     # 并发翻译协程数
```

### 可选本地 Ollama 翻译增强

```toml
[workflow]
llm_translation_enabled = true

[agent.llm]
provider = "local"
api_key  = "ollama"
base_url = "http://host.docker.internal:11434/v1"
model    = "qwen2.5:7b"
```

---

## 完整 config.toml 参考

```toml
# ── 服务 ──────────────────────────────────────────
[server]
host = "0.0.0.0"
port = 8096

# ── 数据库（与 docker-compose.yml 默认值保持一致）──
[database]
type     = "mysql"
host     = "mysql"
port     = 3306
user     = "ytb2bili"
password = "ytb2bili@123"
dbname   = "bili_up"
timezone = "Asia/Shanghai"
auto_migrate = true

# ── 工作流 ────────────────────────────────────────
[workflow]
download_dir             = "./media"
ytdlp_path               = "/usr/local/bin/yt-dlp"
ffmpeg_path              = "/usr/bin/ffmpeg"
llm_translation_enabled  = false         # false = 不调用 LLM，直接用原字幕
llm_translation_batch_size   = 25
llm_translation_max_workers  = 3
llm_translation_source_lang  = "en"
llm_translation_target_lang  = "zh-Hans"

# ── 本地 LLM（可选）─────────────────────────────
[agent.llm]
provider = "local"
api_key  = "ollama"
base_url = "http://host.docker.internal:11434/v1"
model    = "qwen2.5:7b"
```

---

## 常用命令

```bash
# 查看实时日志
docker compose logs -f ytb2bili

# 升级到最新版本
docker compose pull && docker compose up -d

# 停止（保留数据）
docker compose down

# 完全清除（含数据库）
docker compose down -v
```

---

## 支持架构

| 架构 | 适用设备 |
|------|---------|
| `linux/amd64` | 普通 x86 服务器、VPS、PC |
| `linux/arm64` | Apple Silicon Mac（Rosetta-free）、树莓派 4/5、ARM 服务器 |

---

## 相关链接

- 源码仓库：[difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
- Docker 配置仓库：[difyz9/ytb2bili-docker](https://github.com/difyz9/ytb2bili-docker)
- 问题反馈：[GitHub Issues](https://github.com/difyz9/ytb2bili/issues)
