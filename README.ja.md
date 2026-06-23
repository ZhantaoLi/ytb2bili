# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [한국어](README.ko.md)

`ytb2bili` は、YouTube 動画をローカルで準備し、Bilibili へ投稿するためのワークフローサービスです。現在の構成は Go バックエンド、Next.js 管理画面、タスクチェーン、yt-dlp ダウンロード、字幕生成、無料字幕翻訳、任意の AI メタデータ生成、Bilibili 投稿です。

デフォルトの処理は有料 API を必要としません。外部翻訳 API や大規模モデル API は、利用者が自分でエンドポイントとキーを設定した場合だけ有効になります。

## クイックスタート

Docker Compose ファイルは `docker/` 配下にあります。

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# 初回起動前に、下の現在の設定例に合わせて config.toml を修正してください。
docker compose up -d
docker compose logs -f ytb2bili
```

ローカルバックエンド:

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
cp config.toml.example config.toml
# 初回起動前に、下の現在の設定例に合わせて config.toml を修正してください。
go mod download
go run main.go
```

フロントエンド開発:

```bash
cd web
npm install
npm run dev
```

統合アプリは `http://localhost:8096`、フロントエンド開発サーバーは `http://localhost:3000` です。

初回ログイン前に管理者情報を明示的に設定してください。

```bash
YTB2BILI_ADMIN_USERNAME=owner
YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
YTB2BILI_ADMIN_EMAIL=owner@example.local
YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

## 実行時の注意

- `CONFIG_FILE` が設定されていればそのファイルを読み、未設定なら `config.toml` を読みます。
- `go run main.go` は HTTP サーバー、DB マイグレーション、準備タスク、アップロードスケジューラーを起動します。
- ローカル検証では `auto_upload = false` を推奨します。動画は投稿前の準備完了状態で止まります。
- Bilibili 投稿には QR ログインと有効なアカウント cookie が必要です。
- YouTube ダウンロードには proxy や cookies が必要になる場合があります。

## 最小設定

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

[ProxyConfig]
use_proxy = false
proxy_host = "http://127.0.0.1:10809"

[WhisperConfig]
enabled = true
language = "en"
model_path = ""
threads = 0
```

Docker の MySQL 設定例:

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

## 設定項目

- `listen`, `environment`, `debug`, `fileUpDir`, `data_path`, `yt_dlp_path`, `auto_upload`, `primary_ai_service`.
- `[database]`: `type`, `host`, `port`, `username`, `password`, `database`, `ssl_mode`, `timezone`.
- `[auth]`, `[api_auth]`: 管理者ログイン、JWT、任意の署名付き API 認証。
- `[ProxyConfig]`: yt-dlp と一部 HTTP クライアントの proxy 設定。
- `[WhisperConfig]`: 歴史的な名前ですが、`enabled = true` は Bcut 字幕分岐を選択します。Mimo ASR は使用しません。
- `[DeepLXConfig]`, `[OpenAICompatibleConfig]`, `[DeepSeekTransConfig]`, `[GeminiConfig]`: 利用者が自分で設定する任意の翻訳・メタデータ API。
- `[BilibiliConfig]`: タイトル、説明、テンプレート、区分、著作権、投稿オプション。
- `[TenCosConfig]`, `[AnalyticsConfig]`, `[TranslatorConfig]`, `[BaiduTransConfig]`: 高度な任意設定。

## 無料フロー

字幕翻訳は設定済みプロバイダーを優先し、未設定なら無料 Bing 翻訳、失敗時に Google へフォールバックします。メタデータ生成は Gemini、OpenAI 互換 API、DeepSeek を試し、未設定なら元動画のタイトル・説明または動画 ID を使います。

字幕生成では `[WhisperConfig].enabled = true` を設定すると Bcut 分岐を使います。この分岐は YouTube 字幕、既存字幕、Bcut ASR の順で試します。

## ビルドと検証

```bash
make build
make build-web
make build-api
make build-prod
make quick-build
make test
```

```bash
go test -timeout=60s ./...
cd web && npx tsc --noEmit
cd web && npm run lint
cd web && npm run build:prod
git diff --check
```

## ライセンス

[MIT License](LICENSE)
