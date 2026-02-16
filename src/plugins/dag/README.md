# DAG Plugin

Versioned DAG plugin development lives under `src/plugins/dag/src_vN`.

## Current State

- Active version: `src_v3`
- UI sections:
1. `dag-table`: query-backed table from DuckDB + DuckPGQ
2. `three`: interactive DAG scene with ranked columns, nested layers, camera views, and live graph mutations
- Runtime model highlights:
1. Nodes are rank-based and rendered left-to-right by rank.
2. Nodes in the same rank align in one column.
3. Clicking empty space creates a node in the clicked grid cell (spreadsheet-like snapping).
4. If a node receives input from a higher-ranked node, it auto-shifts to `max_input_rank + 1`.
5. Nested DAGs load lazily when parent nodes are selected/opened.
6. Nested layers are centered on parent node X/Y and stacked deeper on Z.
7. Same-layer edges are gray; parent-to-nested links are yellow.
8. Each layer has sparse faint dashed guides (no filled background).
9. Three.js XYZ gizmo is always rendered bottom-left.
10. Nested layers are tested across `iso`, `front`, `side`, and `top` camera views.

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
3. Installs UI dependencies for the requested `src_vN`.

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
3. Three.js interaction flow:
- input/output relations on select
- nested navigation + back history
- live node/edge mutation
- rank/grid alignment rules
- nested layer rank correctness
- multi-view camera checks with layer separation

Always inspect `TEST.md` after each run.

## Notes

- `TEST.md` is generated and should be committed with referenced screenshots when tests are part of the change.
- `test.duckdb` is committed at `src/plugins/dag/src_v3/test/test.duckdb`.
- Domain modeling and SQL language are maintained in `src/plugins/dag/DESIGN.md`.
