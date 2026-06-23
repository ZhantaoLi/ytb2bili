# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [日本語](README.ja.md)

`ytb2bili`는 YouTube 영상을 로컬에서 준비하고 Bilibili에 게시하기 위한 워크플로 서비스입니다. 현재 구성은 Go 백엔드, Next.js 관리자 UI, 작업 체인 스케줄러, yt-dlp 다운로드, 자막 생성, 무료 자막 번역, 선택적 AI 메타데이터 생성, Bilibili 업로드입니다.

기본 흐름은 유료 API가 필요 없습니다. 외부 번역 API나 모델 API는 사용자가 직접 엔드포인트와 키를 설정한 경우에만 활성화됩니다.

## 빠른 시작

Docker Compose 파일은 `docker/` 디렉터리에 있습니다.

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili/docker
cp config.toml.example config.toml
# 처음 시작하기 전에 아래의 현재 설정 예시에 맞게 config.toml을 수정하세요.
docker compose up -d
docker compose logs -f ytb2bili
```

로컬 백엔드:

```bash
git clone https://github.com/ZhantaoLi/ytb2bili.git
cd ytb2bili
cp config.toml.example config.toml
# 처음 시작하기 전에 아래의 현재 설정 예시에 맞게 config.toml을 수정하세요.
go mod download
go run main.go
```

프런트엔드 개발:

```bash
cd web
npm install
npm run dev
```

통합 앱은 `http://localhost:8096`, 프런트엔드 개발 서버는 `http://localhost:3000`입니다.

첫 관리자 로그인 전에 다음 환경 변수를 설정하세요.

```bash
YTB2BILI_ADMIN_USERNAME=owner
YTB2BILI_ADMIN_PASSWORD=change-me-to-a-strong-password
YTB2BILI_ADMIN_EMAIL=owner@example.local
YTB2BILI_ACCOUNT_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

## 실행 참고

- `CONFIG_FILE`이 있으면 해당 파일을 읽고, 없으면 `config.toml`을 읽습니다.
- `go run main.go`는 HTTP 서버, DB 마이그레이션, 준비 작업 스케줄러, 업로드 스케줄러를 시작합니다.
- 로컬 테스트에서는 `auto_upload = false`를 권장합니다. 준비된 영상은 수동 업로드 상태에서 멈춥니다.
- Bilibili 업로드에는 QR 로그인과 유효한 계정 cookie가 필요합니다.
- YouTube 다운로드에는 proxy나 cookies가 필요할 수 있습니다.

## 최소 설정

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

Docker MySQL 예시:

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

## 설정 항목

- `listen`, `environment`, `debug`, `fileUpDir`, `data_path`, `yt_dlp_path`, `auto_upload`, `primary_ai_service`.
- `[database]`: `type`, `host`, `port`, `username`, `password`, `database`, `ssl_mode`, `timezone`.
- `[auth]`, `[api_auth]`: 관리자 로그인, JWT, 선택적 서명 API 인증.
- `[ProxyConfig]`: yt-dlp 및 일부 HTTP 클라이언트의 proxy 설정.
- `[WhisperConfig]`: 역사적 이름이지만 `enabled = true`는 Bcut 자막 분기를 선택합니다. Mimo ASR은 더 이상 사용하지 않습니다.
- `[DeepLXConfig]`, `[OpenAICompatibleConfig]`, `[DeepSeekTransConfig]`, `[GeminiConfig]`: 사용자가 직접 구성하는 선택적 번역/메타데이터 API.
- `[BilibiliConfig]`: 제목, 설명, 템플릿, 카테고리, 저작권, 업로드 옵션.
- `[TenCosConfig]`, `[AnalyticsConfig]`, `[TranslatorConfig]`, `[BaiduTransConfig]`: 고급 선택 설정.

## 무료 흐름

자막 번역은 구성된 제공자를 먼저 사용하고, 없으면 무료 Bing 번역을 사용하며 실패 시 Google로 폴백합니다. 메타데이터 생성은 Gemini, OpenAI 호환 API, DeepSeek를 시도하고, 없으면 원본 영상 제목/설명 또는 영상 ID를 사용합니다.

자막 생성은 `[WhisperConfig].enabled = true`를 설정하면 Bcut 분기를 사용합니다. 이 분기는 YouTube 자막, 기존 자막, Bcut ASR 순서로 시도합니다.

## 빌드와 검증

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

## 라이선스

[MIT License](LICENSE)
