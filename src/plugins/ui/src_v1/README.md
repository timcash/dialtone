# UI src_v1

`ui src_v1` is the shared UI shell and fixture test surface used by Dialtone plugin UIs.

It has two main jobs:
- provide a reusable browser UI shell from `src/plugins/ui/src_v1/ui`
- provide a fixture app and end-to-end test suite from `src/plugins/ui/src_v1/test`

## Use The Wrapper Commands

Use the wrapper form from the repo root:

```sh
./dialtone.sh ui src_v1 <command> [args]
```

Do not prefix ad-hoc env vars for normal use. The wrapper already loads shared configuration from:

```text
env/dialtone.json
```

That file should hold the repo root, managed tool paths, Chrome headed/headless settings, and browser pacing.

## Common Commands

```sh
./dialtone.sh ui src_v1 install
./dialtone.sh ui src_v1 fmt
./dialtone.sh ui src_v1 fmt-check
./dialtone.sh ui src_v1 lint
./dialtone.sh ui src_v1 build
./dialtone.sh ui src_v1 dev
./dialtone.sh ui src_v1 test
```

## Recommended Test Flows

Run the full suite:

```sh
./dialtone.sh ui src_v1 test
```

Run the full suite against a headed remote Chrome session:

```sh
./dialtone.sh ui src_v1 test --attach legion
```

Run a single step when debugging:

```sh
./dialtone.sh ui src_v1 test --attach legion --filter ui-build-and-go-serve
```

For this workspace, `--attach legion` is the normal WSL -> Windows headed browser path.

## What The Full Suite Covers

The full `ui src_v1 test` run currently exercises:
- fixture quality gates: install, format check, lint, build
- fixture boot and smoke navigation
- per-section menu navigation checks for the shared sections
- screenshots and report generation

Some legacy section checks intentionally report `skipped` when the fixture no longer exposes that older dedicated section shape. That is expected and still counts as a passing suite result.

## Headed Browser Behavior

The attach flow uses `chrome src_v3` on the target host. The usual setup is:

```sh
./dialtone.sh ui src_v1 test --attach legion
```

Important shared config lives in `env/dialtone.json`, for example:
- `DIALTONE_CHROME_SRC_V3_HEADLESS`
- `DIALTONE_CHROME_SRC_V3_ACTIONS_PER_SECOND`
- `DIALTONE_REPO_ROOT`
- `DIALTONE_ENV`

This repo currently uses:

```json
"DIALTONE_CHROME_SRC_V3_ACTIONS_PER_SECOND": "5.0"
```

If you want a visible browser, keep:

```json
"DIALTONE_CHROME_SRC_V3_HEADLESS": "0"
```

## Reading Wrapper Output

`./dialtone.sh ui src_v1 test ...` usually runs through the local REPL/subtone path.

That means:
- terminal output stays high-level
- detailed test logs go to the subtone log

Useful commands:

```sh
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

Typical success summary:

```text
DIALTONE> ui test: preparing remote chrome session on legion
DIALTONE> ui test: ensuring chrome src_v3 role=dev on legion
DIALTONE> ui test: running 11 suite steps
DIALTONE> ui test: suite passed
```

## Developing Against The Fixture

Start the fixture dev server:

```sh
./dialtone.sh ui src_v1 dev
```

When you want a remote headed browser while developing:

```sh
./dialtone.sh ui src_v1 dev --browser-node legion
```

## Using The Shared UI Library

Import the shared shell:

```ts
import { setupApp } from '@ui/ui';
```

Create the app shell:

```ts
const { sections, menu } = setupApp({
  title: 'dialtone.myplugin',
  debug: true,
});
```

Register starter-shell sections:

```ts
import { registerUISharedSections } from '@ui/templates';

registerUISharedSections({
  sections,
  menu,
  entries: [
    { sectionID: 'myplugin-home-docs', template: 'docs', title: 'Overview' },
    { sectionID: 'myplugin-runs-table', template: 'table', title: 'Runs' },
    { sectionID: 'myplugin-log-terminal', template: 'terminal', title: 'Signals' },
  ],
});
```

Available shared templates:
- `docs`
- `table`
- `three`
- `terminal`
- `camera`

## Naming Rules

- Section ids use: `<plugin>-<subname>-<underlay-type>`
- Primary underlay kinds are: `docs`, `table`, `three`, `terminal`, `camera`
- Keep controls in overlays, not mixed into the underlay
- Prefer the shared templates before creating a custom shell

## Useful Paths

- shared UI shell:
  - `src/plugins/ui/src_v1/ui`
- fixture app:
  - `src/plugins/ui/src_v1/test/fixtures/app`
- UI suite entrypoint:
  - `src/plugins/ui/src_v1/test/cmd/main.go`
- screenshots:
  - `src/plugins/ui/src_v1/test/screenshots`
- suite reports:
  - `src/plugins/ui/src_v1/TEST.md`
  - `src/plugins/ui/src_v1/TEST_RAW.md`
