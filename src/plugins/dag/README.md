# DAG Plugin

Versioned DAG plugin development lives under `src/plugins/dag/src_vN`.

## Next Steps

1. Keep controls thumb-friendly and single-purpose on mobile.
2. Keep link/unlink interaction driven by node selection history.
3. Keep camera framing readable after add/link/unlink/nest/back interactions.
4. Keep nested visibility tied to selection history (`parent node in history => child layer visible`).
5. Keep tests aligned with the live UI workflow and state model.

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
   - rows 1-2: 6 action buttons
   - row 3: label input + rename button
6. Primary controls use a fixed thumb grid plus node taps:
   - `DAG Back`
   - `DAG Add`
   - `DAG Connect` (link second-most-recent -> most-recent selected node)
   - `DAG Unlink` (remove second-most-recent -> most-recent selected node edge)
   - `DAG Nest`
   - `DAG Clear` (clears node selection + selection history)
7. A top-left node history panel shows the last 5 selected nodes (most recent first, top-to-bottom).
   - Most recent selected node is light blue.
   - Second-most-recent selected node is blue.
   - All other nodes are gray.
8. Section switching uses the shared app menu (`Table` / `Three`).
9. Link workflow uses selection history:
   - select node A
   - select node B
   - tap `DAG Connect` to create directed edge A -> B
10. Rank rule on link creation:
   - if input node rank is less than or equal to any output rank feeding it, input node shifts to `max(output_rank)+1`.
11. Unlink workflow uses selection history:
   - select output/input nodes as above
   - tap `DAG Unlink` to remove that specific directed edge.
12. Nested layers are created/entered from selected nodes using `DAG Nest` and stacked deeper on Z.
13. `Back` behavior is combined:
   - if nested-layer history exists, it undives layer history and reframes camera
   - otherwise, it pops node selection history and moves camera focus to the previous selected node
14. Layer visibility is history-driven:
   - active layer is always visible
   - nested layer is visible only while its `parentNodeId` is present in node history
   - if parent node drops out of history (e.g. `Back` pop), that nested layer hides
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
- build root input/output graph via selection history + `DAG Connect`
- create nested layer and dive
- build deeper nested layers and undive through history with layer visibility constraints
- rename labels from text input
- unlink specific directed edges via selection history
- verify camera remains readable/zoomed out across transitions

Always inspect `TEST.md` after each run.

## Notes

- `TEST.md` is generated and should be committed with referenced screenshots when tests are part of the change.
- `test.duckdb` is committed at `src/plugins/dag/src_v3/test/test.duckdb`.
- Domain modeling and SQL language are maintained in `src/plugins/dag/DESIGN.md`.
