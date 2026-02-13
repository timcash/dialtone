# Template Plugin

Reference plugin for versioned template development under `src/plugins/template/src_vN`.

## Architecture Dependencies (Important)

- `src/libs/ui_v2` is a required shared dependency.
  - Provides `SectionManager`, `Menu`, and base UI shell styles.
  - Template UI versions import this directly.
- `src/libs/test_v2` is a required shared dependency.
  - Provides suite runner/report generation + browser automation actions.
  - Template tests use this to produce `TEST.md`, `test.log`, `error.log`, and screenshots.

If either library changes, all `src_vN` template versions may be affected.

## Current Version

- Current latest template version is `src_v3`.
- Each `src_vN` should contain:
  - `cmd/` Go server entrypoint
  - `ui/` Vite UI
  - `test/` test_v2 suite and generated `TEST.md`

## Prerequisites

- Go
- Bun
- Chrome (used by dev attach and browser tests)

## Quick Start (`src_v3`)

```bash
# Install UI deps for this version (required before lint/dev/test)
./dialtone.sh template install src_v3

# Start dev mode (Vite + debug browser attach)
./dialtone.sh template dev src_v3

# Open in browser
# http://127.0.0.1:3000/
```

If you skip install, typical errors are:
- `tsc: command not found`
- `vite: command not found`

## Daily Dev Flow

```bash
# 1) Ensure deps exist
./dialtone.sh template install src_v3

# 2) Run dev server + browser attach
./dialtone.sh template dev src_v3

# 3) Optional: UI-only server (no debug browser attach)
./dialtone.sh template ui-run src_v3
```

## Test Flow (Must Review `TEST.md` Every Run)

```bash
# Run full suite for the version
./dialtone.sh template test src_v3
```

After every test run, check:

```bash
# Primary report (step-by-step pass/fail + embedded screenshot links)
cat src/plugins/template/src_v3/test/TEST.md

# Full runner log
cat src/plugins/template/src_v3/test/test.log

# Error-focused log
cat src/plugins/template/src_v3/test/error.log
```

What `template test` executes:
1. Preflight: `fmt`, `vet`, `go-build`, `lint`, `format`, `build`
2. Go server run check
3. UI run check
4. Browser section checks (`hero`, `docs`, `table`, `three`, `xterm`, `video`)
5. Lifecycle/invariant checks
6. Cleanup verification

Artifacts written each run:
- `src/plugins/template/src_v3/test/TEST.md`
- `src/plugins/template/src_v3/test/test.log`
- `src/plugins/template/src_v3/test/error.log`
- `src/plugins/template/src_v3/screenshots/test_step_*.png`

`TEST.md` is intended for GitHub rendering, so commit both report + referenced screenshots.

## Create a New `src_vN`

```bash
# Example: create src_v4 from latest existing src_vN
./dialtone.sh template src --n 4
```

Generator behavior:
- Clones latest template version folder.
- Rewrites internal version references in copied files (for example `src_v3` -> `src_v4`).
- Keeps full section/test wiring so new version is immediately testable.

Required follow-up for a new version:

```bash
# Install deps for new version
./dialtone.sh template install src_v4

# Validate full suite and generate src_v4/test/TEST.md
./dialtone.sh template test src_v4

# Start dev on the new version
./dialtone.sh template dev src_v4
```

## Commands Reference

```bash
./dialtone.sh template install <src_vN>   # bun install for selected version UI
./dialtone.sh template fmt <src_vN>       # go fmt for selected version
./dialtone.sh template vet <src_vN>       # go vet for selected version
./dialtone.sh template go-build <src_vN>  # go build for selected version
./dialtone.sh template lint <src_vN>      # tsc --noEmit
./dialtone.sh template format <src_vN>    # UI format check
./dialtone.sh template build <src_vN>     # UI production build
./dialtone.sh template serve <src_vN>     # Go server on :8080
./dialtone.sh template ui-run <src_vN>    # Vite dev server (default :3000)
./dialtone.sh template dev <src_vN>       # Vite + debug browser attach
./dialtone.sh template test <src_vN>      # full test_v2 suite -> TEST.md
./dialtone.sh template src --n <N>        # create a new src_vN
```

## `src_v3` Test Status

```bash
# Verified full pass
./dialtone.sh template test src_v3
```

- Status: `PASS`
- All 13 steps currently pass end-to-end.

## Troubleshooting

- Missing UI tools (`tsc`/`vite` not found):
  - Run `./dialtone.sh template install <src_vN>`.
- Dev server port conflict (`3000`):
  - Stop existing process on `3000`, rerun `./dialtone.sh template dev <src_vN>`.
- Test server port conflict (`8080`):
  - Stop existing process on `8080`, rerun `./dialtone.sh template test <src_vN>`.
