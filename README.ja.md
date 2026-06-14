# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [한국어](README.ko.md)

`ytb2bili` は、ローカル動画の翻訳再生と YouTube から Bilibili への投稿を支援する処理システムです。Web 管理画面、タスクチェーン制御、字幕処理、AI による文案生成、字幕音声合成、音声と映像の同期プレビュー、Bilibili への動画投稿、字幕アップロードまでを一通り提供します。

動画チュートリアル: https://www.bilibili.com/video/BV1tCRTBBEJo

## 主な機能

- ローカル動画の翻訳字幕生成と吹き替え音声生成、同期再生による確認
- YouTube から Bilibili への一連の処理: 動画ダウンロード、音声抽出、字幕生成、翻訳、メタデータ生成、投稿
- ステップごとに有効化できる設定可能なタスクチェーン
- AI 翻訳、タイトル、概要、タグ生成、音声合成への対応
- QR コードログイン、動画投稿、字幕アップロード、アカウント管理などの Bilibili 連携
- Go バックエンドと Next.js フロントエンドによる管理画面
  - ダークモード対応（テーマ設定を永続保存）
  - モバイル対応レスポンシブデザイン（全画面サイズ対応）
  - 自動アップロードの切り替え（柔軟なアップロード管理）
  - タスク履歴管理（削除機能付き）

## 5 分でできる Docker デプロイ

最も簡単な起動方法は Docker Compose です。デフォルトで次の 2 サービスが起動します。

- `mysql`: タスク、アカウント、実行データの保存
- `ytb2bili`: Web 管理画面と処理サービス

### 1. 前提条件

- Docker
- Docker Compose

### 2. デプロイ用ファイルの取得

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
docker compose up -d
```

### 3. 最小構成

`[database]` の初期設定は `docker-compose.yml` と整合しています。少なくとも次の設定を残してください。

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

# YouTube への接続にプロキシが必要な場合:
# proxy_url = "http://127.0.0.1:7890"
```

### 4. サービス起動

```bash
docker compose up -d
docker compose logs -f
```

起動後に `http://localhost:8096` を開いてください。

### 5. 基本的な使い方

1. 管理画面を開く
2. Bilibili アプリで QR コードログインする
3. タスクを新規作成して動画リンクを貼り付ける
4. ダウンロード、処理、投稿が完了するまで待つ

よく使うコマンド:

```bash
docker compose ps
docker compose restart
docker compose down
```

Docker 関連の詳細は [docker/README.md](docker/README.md) を参照してください。

## アーキテクチャ

このプロジェクトは主に 3 つの部分で構成されています。

- 処理エンジン: Go バックエンドがダウンロード、文字起こし、翻訳、メタデータ生成、音声合成、Bilibili 投稿、字幕アップロードを担当
- 管理画面: Next.js フロントエンドがタスク管理、設定、アカウント管理、手動アップロード、同期プレビューを担当
- 実行基盤: `config.toml`、データベース、Docker 関連ファイル、ドキュメント

## リポジトリ構成

- `internal/`: バックエンド本体、タスクチェーン、処理ロジック、サービス構成
- `pkg/`: 再利用可能なパッケージ、Bilibili 連携、ユーティリティ、モデル
- `web/`: Web 管理画面のフロントエンド
- `docker/`: Docker ビルド、実行、デプロイ用ファイル
- `bin/`: サンプルスクリプト、テスト補助、ワークフローファイル

## ローカル開発

### 1. リポジトリを取得

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
```

### 2. 設定ファイルを準備

```bash
cp config.toml.example config.toml
```

データベース、ダウンロード先、API キー、プロキシなどを必要に応じて設定します。まずは [config.toml.example](config.toml.example) を確認してください。

### 3. バックエンド起動

```bash
go mod download
go run main.go
```

### 4. フロントエンド起動

```bash
cd web
npm install
npm run dev
```

### 5. 利用の流れ

1. Web 管理画面を開く
2. Bilibili アカウントでログインする
3. ローカル動画をアップロードして翻訳字幕と吹き替え音声を生成し、同期再生で確認する
4. Chrome 拡張をインストールする: https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest
5. 任意の YouTube 動画ページを開き、拡張からリンクを送信する
6. 管理画面でタスク状況とログを確認する
7. 必要に応じてステップ再実行、文案編集、手動投稿を行う

## 処理フロー

1. ローカル動画を取り込む、または YouTube リンクを送信する
2. 動画とサムネイルをダウンロードする
3. 音声を抽出する
4. 字幕を生成する
5. 字幕を翻訳する
6. 吹き替え音声を合成し、同期再生で確認する
7. タイトル、概要、タグを生成する
8. Bilibili に動画を投稿する
9. Bilibili に字幕をアップロードする

## 設定とビルド

メインの設定ファイルは `config.toml` です。

```bash
cp config.toml.example config.toml
```

主な設定項目:

- `server.port`: サービスのポート
- `database.*`: データベース接続情報
- `workflow.*`: ダウンロード先、プロキシ、ffmpeg、yt-dlp、無料字幕生成、任意のローカル翻訳、アップロードパイプライン設定
- `agent.llm.*`: 任意の API / ローカル互換設定。デフォルト無効で、利用者が自分で設定します
- `updater.enabled`: 自動更新フラグ

ビルドコマンド:

```bash
make build
make build-dev
make build-linux-arm64
make build-all
```

パイプラインを拡張する場合は、まず `internal/chain_task/` 配下の実装を確認してください。

## 検証コマンド

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## ライセンスと連絡先

- ライセンス: [MIT License](LICENSE)
