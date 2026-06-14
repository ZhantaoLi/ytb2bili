# ytb2bili

[English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

`ytb2bili` 是一个面向本地视频翻译播放与 YouTube 到 Bilibili 搬运的处理系统，提供 Web 管理后台、任务链编排、字幕处理、AI 文案生成、字幕音频合成、音视频同步播放、B 站上传、字幕上传等完整能力。

视频教程：https://www.bilibili.com/video/BV1tCRTBBEJo

## 核心特性

- 本地视频翻译播放：支持本地视频生成翻译字幕与配音，并以音视频同步方式播放和校对
- YouTube 到 B 站全流程处理：下载视频、提取音频、转录字幕、翻译字幕、生成元数据、上传 B 站
- 可配置任务链：各步骤可以按用户设置启停，前后端对齐执行语义
- AI 能力集成：支持 AI 翻译、标题、简介、标签生成、字幕音频合成、翻译后配音
- B 站集成：支持扫码登录、视频投稿、字幕上传、账号状态管理
- 现代 Web 管理后台：Go 后端加 Next.js 前端，支持任务查看、重跑、手动上传等操作
  - 深色模式：支持浅色/深色主题切换，偏好本地持久化
  - 移动端适配：响应式设计，支持手机、平板、桌面端全屏幕尺寸
  - 自动上传开关：灵活控制上传时机，支持手动与自动上传切换
  - 任务历史管理：支持删除已完成或失败的任务记录

## 5 分钟 Docker 部署

推荐使用 Docker Compose 直接启动服务，默认会启动两个服务：

- `mysql`：持久化任务、账号和运行数据
- `ytb2bili`：Web 管理后台和搬运服务

### 1. 准备环境

- Docker
- Docker Compose

### 2. 获取部署文件

```bash
git clone https://github.com/difyz9/ytb2bili-docker.git
cd ytb2bili-docker
docker compose up -d
```

### 3. 使用最小配置启动

默认情况下，`[database]` 配置已经和 `docker-compose.yml` 对齐，通常不用改。至少保留下面这段：

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

# 如果你的网络访问 YouTube 需要代理，再补：
# proxy_url = "http://127.0.0.1:7890"
```

### 4. 启动服务

```bash
docker compose up -d
docker compose logs -f
```

服务正常启动后，打开 `http://localhost:8096`

### 5. 开始使用

1. 进入后台
2. 用 B 站 App 扫码登录
3. 新建任务并粘贴视频链接
4. 等待系统自动下载、处理并上传

常用命令：

```bash
docker compose ps
docker compose restart
docker compose down
```

如需 Docker 相关补充说明，可查看 [docker/README.md](docker/README.md)。

## 技术架构

项目由三部分组成：

- 处理引擎：Go 后端负责下载、转录、翻译、元数据生成、字幕音频合成、B 站上传、字幕上传等任务链执行
- 管理后台：Next.js 前端提供任务面板、配置页面、账号管理、手动上传、本地视频翻译播放与同步校对等操作界面
- 运行支撑：通过 `config.toml`、数据库、Docker 部署文件和文档体系支撑本地开发与生产运行

## 项目结构

仓库中的核心目录：

- `internal/`：后端应用代码，包括任务链、处理逻辑和服务装配
- `pkg/`：可复用模块，包括 B 站集成、工具实现和数据模型
- `web/`：Web 管理后台前端代码
- `docker/`：Docker 构建、运行和部署文件
- `bin/`：示例脚本、测试辅助和工作流文件

## 本地开发

### 1. 获取代码

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. 准备配置

```bash
cp config.toml.example config.toml
```

按需填写数据库、下载目录、代理等配置。默认免费流程不需要 API Key；如需启用外部 API 或本地兼容 API，必须由使用者自行配置。常用配置入口见 [config.toml.example](config.toml.example)。

### 3. 启动后端

```bash
go mod download
go run main.go
```

### 4. 启动前端

```bash
cd web
npm install
npm run dev
```

### 5. 使用流程

1. 打开 Web 管理后台
2. 登录 B 站账号
3. 上传本地视频，生成翻译字幕与配音，并在后台进行音视频同步播放和校对
4. 安装 Chrome 插件：https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest
5. 安装插件后，打开任意 YouTube 视频页面，点击插件图标提交视频链接
6. 在管理后台查看任务链执行状态与日志
7. 在需要时重跑步骤、修改文案或手动投稿到 B 站

## 核心流程

1. 导入本地视频或提交 YouTube 链接
2. 下载视频与缩略图
3. 提取音频
4. 生成字幕
5. 翻译字幕
6. 合成翻译配音并进行音视频同步播放
7. 生成标题、简介、标签
8. 上传 B 站视频
9. 上传 B 站字幕

## 配置与构建

项目使用 `config.toml` 作为主要运行配置。开始前可先执行：

```bash
cp config.toml.example config.toml
```

常用配置项包括：

- `server.port`：服务端口
- `database.*`：数据库连接信息
- `workflow.*`：下载目录、代理、ffmpeg、yt-dlp、免费字幕生成、可选本地翻译、上传链路等流程配置
- `agent.llm.*`：可选 API/本地兼容接口配置；默认不启用，需要使用者自行配置
- `DeepLXConfig`：可选 DeepLX 字幕翻译；默认关闭，`endpoint` 必须自行配置
- `OpenAICompatibleConfig`：可选 OpenAI 兼容字幕翻译；默认关闭，`base_url`、`model`、`api_key` 必须自行配置
- `updater.enabled`：自动更新开关

常用构建命令：

```bash
make build
make build-dev
make build-linux-arm64
make build-all
```

如果你要扩展流程，优先查看 `internal/chain_task/` 下已有的处理步骤实现。

## 验证命令

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## 许可证与联系方式

- 许可证：[MIT License](LICENSE)
- GitHub：[@difyz9](https://github.com/difyz9)
- 项目链接：[https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
- QQ 交流群：773066052

<div align="center">

<img src="img/220421_706.png" alt="QQ群二维码" width="180"/>
<img src="img/751763091471.jpg" alt="微信二维码" width="180"/>

</div>
