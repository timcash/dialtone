# Template Plugin

Reference plugin for versioned template development under `src/plugins/template/src_vN`.

## Current Focus

- Latest version is `src_v3`.
- `src_v3` includes:
  - Go server (`cmd/main.go`)
  - UI app (`ui/`, Vite + TypeScript)
  - Integration test runner (`test/`, outputs `TEST.md`)

## Prerequisites

- Go installed
- Bun installed
- Chrome installed (tests/dev browser attach)

## Quick Start (`src_v3`)

```bash
# Install UI dependencies (required before lint/dev/test)
./dialtone.sh template install src_v3

# Run dev mode (starts Vite + launches debug browser)
./dialtone.sh template dev src_v3
```

Without `template install`, commands that invoke UI tooling fail with `tsc: command not found` or `vite: command not found`.

## Commands

```bash
./dialtone.sh template install src_v3   # bun install for src_v3/ui
./dialtone.sh template fmt src_v3       # go fmt
./dialtone.sh template vet src_v3       # go vet
./dialtone.sh template go-build src_v3  # go build
./dialtone.sh template lint src_v3      # tsc --noEmit
./dialtone.sh template format src_v3    # placeholder format check
./dialtone.sh template build src_v3     # vite build
./dialtone.sh template serve src_v3     # go server on :8080
./dialtone.sh template ui-run src_v3    # vite dev (default :3000)
./dialtone.sh template dev src_v3       # dev session + browser attach
./dialtone.sh template test src_v3      # full test_v2 suite -> TEST.md
```

## `src_v3` Test Status

Validated on **February 13, 2026** with:

```bash
./dialtone.sh template test src_v3
```

Latest generated report: `src/plugins/template/src_v3/test/TEST.md`

- Preflight (`fmt`, `vet`, `go-build`, `lint`, `format`, `build`): PASS
- Steps 02-09 (Go run, UI run, hero/docs/table/three): PASS
- First failing step: `10 Xterm Section Validation`
- Failure message:
  - `aria-label Xterm Terminal is outside viewport (rect top=0.0 left=0.0 bottom=744.0 right=1297.0 viewport=1280.0x800.0)`

This means the suite is mostly healthy, but currently exits non-zero at Xterm viewport validation.

## Artifacts

`./dialtone.sh template test src_v3` updates:

- `src/plugins/template/src_v3/test/TEST.md`
- `src/plugins/template/src_v3/test/test.log`
- `src/plugins/template/src_v3/test/error.log`
- `src/plugins/template/src_v3/screenshots/test_step_*.png`
