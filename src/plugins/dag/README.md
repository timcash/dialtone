# Log 2026-02-16 13:36:14 -0800

- Domain terms are standardized for DAG `src_v3`: `menu`, `thumb`, `legend`, and primary overlay (`stage` or `table`) are the section overlay model.
- `Nested` now refers to layers: a node may own many nested layers; each nested layer has exactly one parent node; each layer has many nodes.
- Layer open/close behavior is model language, not implementation detail:
  - opening a layer shows that layer above the parent in the 3D stack
  - closing a layer hides that layer, removes those layer nodes from history, and moves camera focus to the parent node
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

The plugin has two UI sections:

- `dag-table`: query-backed table view (DuckDB + DuckPGQ).
- `three`: interactive DAG stage (mobile-first thumbs, legend history, rename flow, camera modes).

## Domain Language (DAG)

Use these terms consistently in code, tests, and docs.

- `Layer`: one DAG plane of nodes and edges. `root` is the top layer.
- `Nested Layer`: a layer owned by a parent node in another layer.
- `Parent Node`: node that owns/anchors nested layers.
- `History`: stack of layer snapshots used for open/close layer transitions (`DAG Nest` / `DAG Back`).
- `Selection History`: recent node picks in current workflow order (most recent first).
- `Output Node` / `Input Node`: for a directed edge `output -> input`.
- `Rank`: horizontal column index; graph renders left-to-right by rank.
- `Row`: vertical slot within a rank.
- `Visible Layer Set`: active layer + nested layers whose parent nodes are still present in node selection history.
- Model rule: a node may own many nested layers; each layer has exactly one parent node; each layer can contain many nodes.

## UI Language (Buttons + Inputs)

Use this overlay language across docs/code/tests:

- `menu`: global section navigation overlay.
- `stage`: the visualization surface (for DAG, the Three.js canvas).
- `thumb`: button/input controls (includes the rename input).
- `legend`: non-interactive helper area (history, stats, logs, status).

Each section should be modeled with four overlays:
- `menu` (global)
- `thumb`
- `legend`
- one primary overlay kind (`stage` for DAG `three`, `table` for DAG `dag-table`; other UIs may use `xterm` or `docs`)

Menu behavior:

- opening `menu` hides current-section `thumb`;
- `menu` is a fullscreen modal;
- menu button layout uses CSS grid;
- modal menu content targets minimum width `400px` (clamped to viewport);
- menu buttons are centered vertically.

Three-stage controls are driven by exact `aria-label` values.

Primary actions:

- `DAG Back`: close current opened nested layer if layer history exists; otherwise pop node selection history.
- `DAG Add`: create node in active layer.
- `DAG Connect`: create edge from second-most-recent selected node to most-recent selected node.
- `DAG Unlink`: remove that specific directed edge.
- `DAG Nest`: create/open nested layer from selected node.
- `DAG Clear Picks`: clear current node selection + selection history.

Camera actions:

- `DAG Camera Z`: top-down map style.
- `DAG Camera ISO`: isometric view.
- `DAG Camera Side`: side view to reveal layer depth/nested link structure.

Label actions:

- `DAG Label Input`: text for selected node label.
- `DAG Rename`: apply label rename to selected node.

Display semantics:

- Legend history panel shows last 5 selected nodes (most recent first).
- Node history title includes layer status as `current/visible`.
- Most recent node color: light blue.
- Second-most-recent node color: blue.
- Older nodes: gray.

## Interaction Rules

- Link/unlink always use selection history pair order.
- Creating an edge enforces rank rule:
  - if input rank `<=` any output rank feeding it, input moves to `max(output_rank)+1`.
- Nested visibility is history-driven:
  - active layer is visible;
  - nested layer is visible only while its parent node remains in selection history.
- Closing a layer:
  - hides the closed layer;
  - removes nodes from that layer out of node selection history;
  - moves camera focus to the closed layer's parent node.
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
