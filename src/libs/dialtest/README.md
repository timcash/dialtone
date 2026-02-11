# `dialtest`

`dialtest` is the shared smoke-testing/runtime test library for Dialtone plugins.

## Responsibilities

- Provides reusable smoke runner orchestration (`SmokeRunner`) for `src_vN` plugin folders.
- Runs default preflight checks for Go + UI (format/lint/build/startup probes).
- Captures browser console/errors, screenshots, and emits `SMOKE.md` + `smoke.log`.
- Provides common browser-driven assertions (`NavigateToSection`, `WaitForAriaLabel`, `AssertElementHidden`).
- Integrates with the Chrome plugin API for browser lifecycle.

## Chrome Integration

`dialtest` does not own browser process management logic. It consumes:

- `src/plugins/chrome/app.StartSession(...)`
- `src/plugins/chrome/app.CleanupSession(...)`

Role model used by convention:

- `dev`: headed browser, reusable/persistent profile.
- `smoke`: isolated headless browser, cleaned at the end of a smoke run.

## Default Preflight

For a plugin version directory `src/plugins/<plugin>/src_vN`:

- Go checks in `src_vN`:
  - `go fmt ./...`
  - `go vet ./...`
  - `go build ./...`
  - `go run cmd/main.go` startup probe
- UI checks in `src_vN/ui`:
  - `./dialtone.sh bun exec install`
  - `./dialtone.sh bun exec run lint`
  - `./dialtone.sh bun exec run build`
  - `./dialtone.sh bun exec run dev ...` startup probe
- Source formatting/lint:
  - `./dialtone.sh bun exec x prettier --write ...`
  - `./dialtone.sh bun exec x prettier --check ...`
  - JS/TS source scan excludes `node_modules`, `.pixi`, and `dist`.

## Intended Use

Each plugin `src_vN/smoke/smoke.go` should be mostly scenario definitions and assertions for that version, with minimal setup boilerplate.

## Known Problems

- If UI build/lint tools hang (for example `vite build`), the default smoke `TotalTimeout` (30s) can fire before scenario steps begin.
- This is expected fail-fast behavior: runner panics with stage context (for example `preflight:UI Build`) and exits non-zero.
- In these failures, command output is still streamed live into `src_vN/smoke/smoke.log`, and `SMOKE.md` is written with completed stages.
