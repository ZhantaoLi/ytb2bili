# CLAUDE.md

This file is the shortest useful onboarding guide for agents working in this
repo. Prefer current code and command output over assumptions.

## Project Snapshot

`ytb2bili` is a local-first YouTube-to-Bilibili workflow app.

- Backend: Go, Gin, GORM, Fx, cron schedulers.
- Frontend: Next.js, React, Tailwind, exported as static files.
- Runtime entry: `main.go`.
- Embedded frontend output: `internal/web/bili-up-web`.
- Local data, downloaded media, cookies, logs, and config are not source code.

## Working Rules

1. Start from current state: run `git status --short`, inspect the relevant
   files, and verify the actual config/schema before editing.
2. Do not trigger Bilibili upload, hard-delete database rows, or delete media
   files unless the user explicitly confirms that action.
3. Treat `config.toml`, cookies, API keys, downloaded videos, and `data/` as
   local/private runtime state. Do not commit secrets or generated media.
4. `go run main.go` starts schedulers. It can process queued DB tasks, so use it
   only when runtime side effects are intended.
5. Keep changes narrow. If a fix touches both backend behavior and frontend
   API usage, make the contract explicit and verify both sides.

## Key Paths

- `main.go`: app wiring, migrations, route registration, schedulers, static UI.
- `internal/chain_task`: preparation chain and upload scheduler.
- `internal/chain_task/handlers`: download, subtitles, translation, metadata,
  upload task implementations.
- `internal/core/services`: database-backed business services.
- `internal/handler`: HTTP handlers under `/api/v1`.
- `pkg/store/model`: persisted GORM models.
- `pkg/translator`, `pkg/subtitle`: translation and subtitle utilities.
- `web/src`: frontend source.
- `web/src/lib/api.ts`: frontend API client and auth-aware fetch helper.
- `internal/web/bili-up-web`: committed static frontend served by Go.
- `tools/`: small operational tools; keep them scoped and tested.

## Common Commands

Backend:

```powershell
go test -timeout=60s ./...
```

Frontend:

```powershell
cd web
npx tsc --noEmit
npm run lint
npm run build:prod
```

Full build:

```powershell
make build
```

If `web/src` changes and the Go binary must serve the updated UI, ensure the
exported frontend is reflected in `internal/web/bili-up-web`.

## Change Guidelines

Backend:

- Keep task steps idempotent where possible.
- Persist real failures instead of returning false success.
- Preserve existing status semantics unless the migration is explicit.
- Add or update focused tests near the changed package.

Frontend:

- Use `apiFetch` for protected backend API calls.
- Do not attach admin tokens to third-party URLs.
- Keep generated static output in sync when shipping embedded UI changes.
- Avoid committing local build caches such as `web/tsconfig.tsbuildinfo`.

Task Chain:

- Preparation and upload are separate flows.
- `auto_upload=false` should stop after the prepared/ready state.
- Upload steps must not run silently when prerequisites such as BVID, subtitle
  file, or video file are missing.
- Legacy task-step names may exist in old DB rows; read through the service
  layer before changing display or progress logic.

Config:

- Prefer `config.toml.example` for documented defaults.
- Optional external APIs must stay optional. The free/default path should keep
  working without paid services.
- Proxy settings are runtime config; do not hardcode local proxy assumptions.

## Verification Checklist

Before claiming a change is complete:

1. Run the smallest relevant test first.
2. Run `go test -timeout=60s ./...` for backend changes.
3. Run `npx tsc --noEmit` and `npm run lint` for frontend changes.
4. Run `npm run build:prod` when frontend export or embedded UI may change.
5. Check `git status --short` and keep unrelated runtime artifacts out of the
   commit.

## When Unsure

Read the code path that owns the behavior, then verify with a real command or a
small test. Do not rely on handler names, stale DB rows, old generated files, or
previous conversation memory as proof.
