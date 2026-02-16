# DAG Plugin

Versioned DAG plugin development lives under `src/plugins/dag/src_vN`.

## Next Steps

1. Keep controls as a fixed 3x3 thumb grid for mobile (up to 9 buttons).
2. Keep each button single-purpose.
3. Keep graph interaction language explicit: `output node` -> `input node` for directed links.
4. Keep camera framing readable after every interaction (add/link/nest/back/delete/unlink).
5. Keep nested layer visibility strict: active layer + parent context only.

## Current State

- Active version: `src_v3`
- UI sections:
1. `dag-table`: query-backed table from DuckDB + DuckPGQ
2. `three`: mobile-first interactive DAG scene with thumb controls, nested layers, and renameable node labels
- Runtime model highlights:
1. Nodes are rank-based and rendered left-to-right by rank.
2. Nodes in the same rank align in one column.
3. `src_v3` starts from an empty root DAG; users build graph structure via UI controls.
4. `dag-table` is rendered flush at top-left (no section padding/margins).
5. Three controls are rendered in one unified CSS grid at the bottom:
   - rows 1-3: 9 action buttons
   - row 4: label input + rename button
6. Primary controls use a fixed 9-button grid plus node taps:
   - `DAG Back`
   - `DAG Add`
   - `DAG Pick Output`
   - `DAG Pick Input`
   - `DAG Connect` (link output -> input)
   - `DAG Unlink` (remove output -> input)
   - `DAG Nest`
   - `DAG Delete Node`
   - `DAG Clear Picks`
7. A top-left legend in Three explains node colors and displays live first/second link picks:
   - Selected node
   - First pick (`output`)
   - Second pick (`input`)
   - Inputs to selected node
   - Outputs from selected node
8. Section switching uses one menu button that toggles a modal with `Table` and `Three`.
9. Link workflow is pair-based:
   - select node A, tap `DAG Pick Output`
   - select node B, tap `DAG Pick Input`
   - tap `DAG Connect` to create directed edge A -> B
10. Rank rule on link creation:
   - if input node rank is less than or equal to any output rank feeding it, input node shifts to `max(output_rank)+1`.
11. Unlink workflow is pair-based:
   - select output/input picks as above
   - tap `DAG Unlink` to remove that specific directed edge.
12. Nested layers are created/entered from selected nodes using `DAG Nest` and stacked deeper on Z.
13. `Back` undives through layer history, reframes camera, and hides deeper nested layers.
14. Node labels are renameable via the bottom input (`DAG Label Input` + `DAG Rename`) and update in-scene.
15. Camera framing is intentionally zoomed out for mobile readability.

## Folder Layout

- `src/plugins/dag/cli/`
- `src/plugins/dag/src_v3/cmd/`
- `src/plugins/dag/src_v3/ui/`
- `src/plugins/dag/src_v3/test/`
- `src/plugins/dag/src_v3/screenshots/`
- `src/plugins/dag/src_v3/test/test.duckdb`
- `src/plugins/dag/DESIGN.md`

## Install And Dependencies

`dag install` does more than UI npm/bun install:
1. Ensures managed Go/Bun requirements.
2. Ensures DuckDB CLI exists in `DIALTONE_ENV` (idempotent).
3. Runs a version install hook when present (for `src_v3`, this ensures `github.com/marcboeker/go-duckdb` is downloaded).
4. Installs UI dependencies for the requested `src_vN`.

```bash
./dialtone.sh dag install src_v3
```

DuckDB is managed in `DIALTONE_ENV` for DAG workflows. OS-level `duckdb` is not required.

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
./dialtone.sh dag test <src_vN>      # Run full test_v2 suite and regenerate TEST.md
./dialtone.sh dag smoke <src_vN>     # Legacy smoke test (if present)
./dialtone.sh dag src --n <N>        # Create new src_vN from latest DAG version
./dialtone.sh dag help               # Print DAG command help
```

## Daily Workflow

```bash
./dialtone.sh dag install src_v3
./dialtone.sh dag dev src_v3
```

App URLs:
- Dev UI (vite): `http://127.0.0.1:3000/`
- Served build (go): `http://127.0.0.1:8080/`

## Test Workflow

```bash
./dialtone.sh dag test src_v3
```

Test output:
- `src/plugins/dag/src_v3/test/TEST.md`
- `src/plugins/dag/src_v3/test/test.log`
- `src/plugins/dag/src_v3/test/error.log`
- `src/plugins/dag/src_v3/screenshots/test_step_*.png`

Current suite validates:
1. DuckDB graph queries (including nested/input/output query actions).
2. DAG table data + screenshot artifact generation.
3. Three.js user-story workflow end-to-end:
- start from empty DAG and create first node
- build root input/output graph via output/input picks + `DAG Connect`
- create nested layer and dive
- build deeper nested layers and undive through history with layer visibility constraints
- rename labels from text input
- delete nodes and unlink specific directed edges via output/input picks
- verify camera remains readable/zoomed out across transitions

Always inspect `TEST.md` after each run.

## Notes

- `TEST.md` is generated and should be committed with referenced screenshots when tests are part of the change.
- `test.duckdb` is committed at `src/plugins/dag/src_v3/test/test.duckdb`.
- Domain modeling and SQL language are maintained in `src/plugins/dag/DESIGN.md`.
