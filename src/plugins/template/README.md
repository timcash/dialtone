# Template Plugin

Reference plugin for versioned template development under `src/plugins/template/src_vN`.

## Current Focus

- Latest version is `src_v3`.
- `src_v3` includes:
  - Go server (`cmd/main.go`)
  - UI app (`ui/`, Vite + TypeScript)
  - Integration test runner (`test/`, outputs `TEST.md`)

## Shared Libraries

- `src/libs/ui_v2`
  - Shared app shell used by template UI versions.
  - Provides `SectionManager`, `Menu`, and common styles.
  - Current shell defaults match the `www` pattern: hidden header, floating bottom-right menu, modal panel behavior, and scroll lock while menu is open.
- `src/libs/test_v2`
  - Shared test runner and browser automation utilities.
  - Generates `TEST.md`, `test.log`, `error.log`, and step screenshots.
  - Used by `src_vN/test/main.go` suites.

## Prerequisites

- Go installed
- Bun installed
- Chrome installed (tests/dev browser attach)

## Quick Start (`src_v3`)

```bash
# 1) Install UI dependencies (required before lint/dev/test)
./dialtone.sh template install src_v3

# 2) Run dev mode (starts Vite + launches debug browser)
./dialtone.sh template dev src_v3
```

Without `template install`, commands that invoke UI tooling fail with `tsc: command not found` or `vite: command not found`.
Open the app at `http://127.0.0.1:3000/`.

## Create a New `src_vN`

Use the generator to clone the latest template version into a new version folder:

```bash
./dialtone.sh template src --n 4
```

What this now does:
- Clones from the latest existing `src_vN`.
- Rewrites internal version references (for example `src_v3` -> `src_v4`) inside the copied files.
- Keeps all standard sections and tests wired: `hero`, `docs`, `table`, `three`, `xterm`, `video`.
- Produces a version that can run full test flow and generate `TEST.md` in the new folder.

Recommended follow-up for a new version:

```bash
./dialtone.sh template install src_v4
./dialtone.sh template test src_v4
./dialtone.sh template dev src_v4
```

## Standard Workflows

### Dev Workflow

```bash
./dialtone.sh template install src_v3
./dialtone.sh template dev src_v3
```

### Test Workflow

```bash
./dialtone.sh template test src_v3
```

The suite runs:
1. Preflight: `fmt`, `vet`, `go-build`, `lint`, `format`, `build`
2. Go server run check
3. UI run check
4. Browser section checks
5. Lifecycle/invariant checks
6. Cleanup verification

### Build / Serve Workflow

```bash
./dialtone.sh template build src_v3
./dialtone.sh template serve src_v3
```

## Commands Reference

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
./dialtone.sh template src --n <N>      # create new src_vN from latest version
```

## `src_v3` Test Status

Validated on **February 13, 2026** with:

```bash
./dialtone.sh template test src_v3
```

Latest generated report: `src/plugins/template/src_v3/test/TEST.md`

- Status: `PASS`
- All 13 steps currently pass end-to-end.

## Artifacts

`./dialtone.sh template test src_v3` updates:

- `src/plugins/template/src_v3/test/TEST.md`
- `src/plugins/template/src_v3/test/test.log`
- `src/plugins/template/src_v3/test/error.log`
- `src/plugins/template/src_v3/screenshots/test_step_*.png`

`TEST.md` embeds screenshot paths so it renders correctly on GitHub when the matching screenshot files are committed.

## Troubleshooting

- `tsc: command not found` or `vite: command not found`
  - Run `./dialtone.sh template install <src_vN>`.
- Port `3000` busy for `template dev`
  - Stop existing process on that port, rerun `./dialtone.sh template dev <src_vN>`.
- Port `8080` busy during tests
  - Stop existing process on that port, rerun `./dialtone.sh template test <src_vN>`.
