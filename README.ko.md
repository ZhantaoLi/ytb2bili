# ytb2bili

[简体中文](README.zh-CN.md) | [English](README.en.md) | [日本語](README.ja.md)

`ytb2bili`는 로컬 비디오 번역 재생과 YouTube에서 Bilibili로의 업로드를 지원하는 처리 시스템입니다. 웹 관리 화면, 작업 체인 오케스트레이션, 자막 처리, AI 문안 생성, 자막 음성 합성, 오디오와 비디오 동기화 미리보기, Bilibili 업로드, 자막 업로드 기능을 제공합니다.

비디오 튜토리얼: https://www.bilibili.com/video/BV1tCRTBBEJo

## 주요 기능

- 로컬 비디오용 번역 자막 및 더빙 음성 생성, 동기화 재생으로 검수 가능
- YouTube에서 Bilibili까지의 전체 처리 흐름: 다운로드, 오디오 추출, 자막 생성, 번역, 메타데이터 생성, 업로드
- 단계별 활성화 또는 비활성화가 가능한 작업 체인
- AI 번역, 제목, 설명, 태그 생성, 음성 합성 지원
- QR 로그인, 영상 업로드, 자막 업로드, 계정 관리 등 Bilibili 연동
- Go 백엔드와 Next.js 프런트엔드 기반의 웹 관리 화면
  - 다크 모드 지원 (테마 설정 영구 저장)
  - 모바일 반응형 디자인 (모든 화면 크기 지원)
  - 자동 업로드 전환 (유연한 업로드 워크플로)
  - 작업 기록 관리 (삭제 기능 포함)

## 5분 Docker 배포

가장 빠른 시작 방법은 Docker Compose입니다. 기본적으로 다음 두 서비스를 실행합니다.

- `mysql`: 작업, 계정, 실행 데이터 저장
- `ytb2bili`: 웹 관리자와 처리 서비스

### 1. 준비 사항

- Docker
- Docker Compose

### 2. 배포 파일 받기

```bash
git clone https://github.com/difyz9/ytb2bili-docker.git
cd ytb2bili-docker
docker compose up -d
```

### 3. 최소 설정

기본 `[database]` 설정은 `docker-compose.yml`과 맞춰져 있습니다. 최소한 아래 설정은 유지하세요.

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

# YouTube 접속에 프록시가 필요하면 추가:
# proxy_url = "http://127.0.0.1:7890"
```

### 4. 서비스 시작

```bash
docker compose up -d
docker compose logs -f
```

그다음 `http://localhost:8096`를 열면 됩니다.

### 5. 기본 사용 방법

1. 관리자 화면 열기
2. Bilibili 앱으로 QR 로그인
3. 작업을 생성하고 비디오 링크 붙여넣기
4. 다운로드, 처리, 업로드가 끝날 때까지 대기

자주 쓰는 명령:

```bash
docker compose ps
docker compose restart
docker compose down
```

Docker 관련 자세한 내용은 [docker/README.md](docker/README.md)를 참고하세요.

## 아키텍처

이 프로젝트는 크게 세 부분으로 구성됩니다.

- 처리 엔진: Go 백엔드가 다운로드, 전사, 번역, 메타데이터 생성, 음성 합성, Bilibili 업로드, 자막 업로드를 담당
- 관리 화면: Next.js 프런트엔드가 작업 패널, 설정, 계정 관리, 수동 업로드, 동기화 재생 검수를 담당
- 실행 기반: `config.toml`, 데이터베이스, Docker 파일, 문서 체계

## 저장소 구조

- `internal/`: 백엔드 본체, 작업 체인, 처리 로직, 서비스 연결
- `pkg/`: 재사용 가능한 패키지, Bilibili 연동, 유틸리티, 모델
- `web/`: 웹 관리자 프런트엔드
- `docker/`: Docker 빌드, 실행, 배포 파일
- `bin/`: 예제 스크립트, 테스트 보조, 워크플로 파일

## 로컬 개발

### 1. 저장소 클론

```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

### 2. 설정 준비

```bash
cp config.toml.example config.toml
```

데이터베이스, 다운로드 디렉터리, API 키, 프록시 등을 환경에 맞게 채우세요. 시작점으로 [config.toml.example](config.toml.example)을 확인하면 됩니다.

### 3. 백엔드 실행

```bash
go mod download
go run main.go
```

### 4. 프런트엔드 실행

```bash
cd web
npm install
npm run dev
```

### 5. 사용 흐름

1. 웹 관리자 화면 열기
2. Bilibili 계정 로그인
3. 로컬 비디오를 업로드해 번역 자막과 더빙 음성을 생성하고 동기화 재생으로 검수
4. Chrome 확장 설치: https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest
5. 임의의 YouTube 비디오 페이지를 열고 확장에서 링크 제출
6. 관리자 화면에서 작업 상태와 로그 확인
7. 필요 시 단계 재실행, 문안 수정, 수동 업로드 진행

## 처리 흐름

1. 로컬 비디오를 가져오거나 YouTube 링크 제출
2. 비디오와 썸네일 다운로드
3. 오디오 추출
4. 자막 생성
5. 자막 번역
6. 더빙 음성 합성 및 동기화 재생 미리보기
7. 제목, 설명, 태그 생성
8. Bilibili에 비디오 업로드
9. Bilibili에 자막 업로드

## 설정 및 빌드

주요 실행 설정 파일은 `config.toml`입니다.

```bash
cp config.toml.example config.toml
```

주요 설정 항목:

- `server.port`: 서비스 포트
- `database.*`: 데이터베이스 연결 정보
- `workflow.*`: 다운로드 디렉터리, 프록시, ffmpeg, yt-dlp, 무료 자막 생성, 선택적 로컬 번역, 업로드 파이프라인 설정
- `agent.llm.*`: 선택적 API/로컬 호환 설정이며 기본값은 비활성화, 사용자가 직접 구성
- `updater.enabled`: 자동 업데이트 스위치

빌드 명령:

```bash
make build
make build-dev
make build-linux-arm64
make build-all
```

파이프라인을 확장하려면 먼저 `internal/chain_task/` 아래 구현을 확인하세요.

## 검증 명령

```bash
go test ./...
go build -o ytb2bili main.go
curl http://localhost:8096/health
```

## 라이선스 및 연락처

- 라이선스: [MIT License](LICENSE)
- GitHub: [@difyz9](https://github.com/difyz9)
- 프로젝트: [https://github.com/difyz9/ytb2bili](https://github.com/difyz9/ytb2bili)
- QQ 그룹: 773066052
