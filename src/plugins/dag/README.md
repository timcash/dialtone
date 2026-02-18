# Log 2026-02-16 13:36:14 -0800

- Domain terms are standardized for DAG `src_v3`: overlays are `menu`, `mode-form`, `legend`, optional `chatlog`; underlay is `stage` or `table`; together they compose a section.
- `Nested` now refers to layers: a node may own many nested layers; each nested layer has exactly one parent node; each layer has many nodes.
- Layer open/close behavior is model language, not implementation detail:
  - opening a layer shows that layer above the parent in the 3D stack
  - closing a layer hides that layer, removes those layer nodes from history, and keeps parent-node context in focus
- Mode-form controls are mode-based in `src_v3`:
  - bottom-right `DAG Mode` button is always visible and cycles `Build -> Layer -> View` (internal mode ids: `graph -> layer -> camera`)
  - thumb labels/actions update from currently selected node context
- Dev/test language updates:
  - `dag dev src_v3` reuses existing dev server/session when possible
  - `dag test src_v3 --attach` reuses the running headed dev browser session for visible playback

# DAG Plugin

## How To Test (Primary)

```bash
./dialtone.sh dag test src_v3
```

Use this command as the default verification entrypoint for DAG changes.

## What This Plugin Is

Versioned DAG plugin development lives under `src/plugins/dag/src_vN`.

Current active version is `src_v3`.

The plugin has three UI sections:

- `dag-meta-table`: query-backed table view (DuckDB + DuckPGQ).
- `dag-3d-stage`: interactive DAG stage (mobile-first mode-form controls, legend history, rename flow, camera modes).
- `dag-log-xterm`: xterm log section with mode-form overlay for cursor/select/command actions.

Section id naming rule:

- Always use `<plugin-name>-<subname>-<underlay-type>`.
- Examples in DAG `src_v3`: `dag-meta-table`, `dag-3d-stage`, `dag-log-xterm`.

## Domain Language (DAG)

Use these terms consistently in code, tests, and docs.

- `Layer`: one DAG plane of nodes and edges. `root` is the top layer.
- `Nested Layer`: a layer owned by a parent node in another layer.
- `Parent Node`: node that owns/anchors nested layers.
- `History`: stack of layer navigation snapshots used when going back to parent layers.
- `Selection History`: recent node picks in current workflow order (most recent first).
- `Output Node` / `Input Node`: for a directed edge `output -> input`.
- `Rank`: horizontal column index; graph renders left-to-right by rank.
- `Row`: vertical slot within a rank.
- `Visible Layer Set`: root + active layer + nested layers explicitly marked open.
- Model rule: a node may own many nested layers; each layer has exactly one parent node; each layer can contain many nodes.

## UI Language (Buttons + Inputs)

Use this section language across docs/code/tests:

- Overlays:
- `menu`: global section navigation overlay.
- `mode-form`: button/input controls (includes the rename input).
- `legend`: non-interactive helper area (history, stats, logs, status).
- `chatlog` (optional): context thought feed above thumbs (xterm overlay, no background).
- Underlays (exactly one per section):
- `stage`: visualization surface (for DAG `dag-3d-stage`, the Three.js canvas).
- `table`: query-backed table surface (for DAG `dag-meta-table`).
- `docs` | `xterm` | `video`: available underlay kinds for other section types.

Section formula: one underlay + overlays = one section.

Menu behavior:

- opening `menu` hides current-section `mode-form`;
- `menu` is a fullscreen modal and the modal root is `nav`;
- `nav` uses CSS grid directly for menu actions;
- menu buttons are centered vertically.

Three-stage controls are mode-driven, with a persistent `DAG Mode` button.

Layer + DAG calculator layout:

- 8 mode action buttons per mode
- 1 always-visible `DAG Mode` button
- 1 `DAG Label Input` text box
- 1 `DAG Rename` apply button

Table section also uses a mode-form layout:

- 8 mode action buttons
- 1 `Table Mode` button
- 1 `Table Query Input`
- 1 `Table Submit` apply button
- table section layout is two-row grid: table on top, form below

Primary actions:

- `DAG Mode` (always bottom-right): cycle thumb mode between `Build`, `Layer`, and `View`.
- `DAG Back`: go to parent layer if navigation history exists; otherwise pop node selection history.
- `DAG Add`: create node in active layer.
- `DAG Link|Unlink`: one context button label (`Link` or `Unlink`) based on current pair edge state.
- `DAG Open|Close Layer`: one context button label (`Open` or `Close`) bound to selected-node nested-layer state.
  - expected close flow: use `Back` until at the parent node, then press `Close`
- `DAG Clear Picks`: clear current node selection + selection history.

Camera actions:

- `DAG Camera Z`: top-down map style.
- `DAG Camera ISO`: isometric view.
- `DAG Camera Side`: side view to reveal layer depth/nested link structure.

Label actions:

- `DAG Label Input`: text for selected node label.
- `DAG Rename`: apply label rename to selected node.

Mode semantics (8 buttons each):

- `Build` (`graph`): `Back`, `Add`, `Link|Unlink`, `Open|Close`, `Clear`, `Rename`, `Focus`, `Labels On|Off`
- `Layer` (`layer`): `Open|Close`, `Add`, `Link|Unlink`, `Back`, `Clear`, `Rename`, `Focus`, `Labels On|Off`
- `View` (`camera`): `Top`, `Iso`, `Side`, `Focus`, `Labels On|Off`, `Back`, `Add`, `Open|Close`

Log section controls are also mode-driven, with a persistent `Log Mode` button:

- `Cursor`: arrow/home/end cursor movement + `Select` + `Copy`
- `Select`: start/extend selection with arrows, then `Copy`
- `Command`: `Send`/`Clear` for `Log Command Input` plus cursor/select/copy helpers
- `Log Command Input` submits to xterm on Enter
- `Log Submit` submits `Log Command Input` to xterm

Section hash ids:

- `#dag-meta-table`
- `#dag-3d-stage`
- `#dag-log-xterm`

Display semantics:

- Legend history panel shows last 5 selected nodes (most recent first).
- Node history title includes layer status as `current/visible`.
- Most recent node color: glowing white.
- Second-most-recent node color: blue.
- Older nodes: gray.
- Chatlog overlay (xterm) sits above thumbs:
  - appends one line per test thought
  - newest line is on bottom; older lines push upward
  - max visible window is 7 lines
  - older lines fade; lines beyond 7 are dropped
  - click logs use concise aria-form lines, e.g. `USER> click aria-open`

Styling semantics:

- default `button` style is shared across forms and menu:
  - black background
  - blue highlight on hover/focus/press
  - square-ish corners

## Interaction Rules

- Link/unlink always use selection history pair order.
- Creating an edge enforces rank rule:
  - if input rank `<=` any output rank feeding it, input moves to `max(output_rank)+1`.
- Nested visibility is explicit:
  - root layer remains visible
  - active layer remains visible
  - nested layers remain visible while marked open
- Closing a nested layer:
  - hide the selected parent node’s nested layer
  - remove nodes from that layer out of node selection history
  - button text for that selected parent flips from `Close Layer` to `Open Layer`
- Selecting a node re-centers camera on that node while preserving current camera style offset.

## Folder Map (Where To Edit)

- CLI and orchestration:
  - `src/plugins/dag/cli/`
- Server runtime:
  - `src/plugins/dag/src_v3/cmd/`
- UI runtime:
  - `src/plugins/dag/src_v3/ui/`
  - `src/plugins/dag/src_v3/ui/src/components/three/index.ts`
  - `src/plugins/dag/src_v3/ui/src/style.css`
  - `src/plugins/dag/src_v3/ui/index.html`
- Tests:
  - `src/plugins/dag/src_v3/test/`
  - `src/plugins/dag/src_v3/test/TEST.md`
- Test screenshots:
  - `src/plugins/dag/src_v3/screenshots/`
- Domain SQL/model docs:
  - `src/plugins/dag/DESIGN.md`

## Commands

```bash
./dialtone.sh dag install <src_vN>   # Ensure Go/Bun + DuckDB in DIALTONE_ENV + UI deps
./dialtone.sh dag fmt <src_vN>       # go fmt
./dialtone.sh dag vet <src_vN>       # go vet
./dialtone.sh dag go-build <src_vN>  # go build
./dialtone.sh dag lint <src_vN>      # tsc --noEmit
./dialtone.sh dag format <src_vN>    # UI format check
./dialtone.sh dag build <src_vN>     # UI production build
./dialtone.sh dag serve <src_vN>     # Run Go server on :8080
./dialtone.sh dag ui-run <src_vN>    # Run Vite UI dev server
./dialtone.sh dag dev <src_vN>       # Vite + debug browser attach
./dialtone.sh dag test <src_vN>      # Run full test_v2 suite and regenerate TEST.md artifacts
./dialtone.sh dag test <src_vN> --attach  # Reuse headed dev browser for live-visible test playback
./dialtone.sh dag smoke <src_vN>     # Legacy smoke test (if present)
./dialtone.sh dag src --n <N>        # Create new src_vN from latest DAG version
./dialtone.sh dag help               # Print DAG command help
```

Examples:

```bash
./dialtone.sh dag test src_v3
./dialtone.sh dag dev src_v3
./dialtone.sh dag src --n 5
```

## Agent Workflow (Code Changes)

When adding/changing DAG behavior, use this sequence.

1. Confirm target version (`src_v3` unless explicitly creating a new version).
2. Make UI/runtime edits in `src_v3`.
3. Keep terminology and button semantics aligned with this README.
4. Run full test command:
   - `./dialtone.sh dag test src_v3`
5. Inspect and commit generated artifacts when behavior changed:
   - `src/plugins/dag/src_v3/test/TEST.md`
   - `src/plugins/dag/src_v3/screenshots/test_step_*.png`
   - `src/plugins/dag/src_v3/test/test.duckdb` (if changed by suite)
6. Update tests in `src/plugins/dag/src_v3/test/*.go` when behavior expectations change.
7. Re-run `./dialtone.sh dag test src_v3` and ensure all steps pass.

## Test Outputs

Running DAG tests produces:

- `src/plugins/dag/src_v3/test/TEST.md`
- `src/plugins/dag/src_v3/test/test.log`
- `src/plugins/dag/src_v3/test/error.log`
- `src/plugins/dag/src_v3/screenshots/test_step_*.png`

Always review `TEST.md` before finalizing changes.

## Dependencies

`dag install` does more than UI dependency install.

1. Ensures managed Go/Bun requirements.
2. Ensures DuckDB CLI exists in `DIALTONE_ENV` (idempotent).
3. Runs version install hook when present (for `src_v3`, downloads `github.com/marcboeker/go-duckdb`).
4. Installs UI deps for requested `src_vN`.

DuckDB is managed in `DIALTONE_ENV`; OS-level `duckdb` is not required.
