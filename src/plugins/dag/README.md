# DAG Plugin

Versioned DAG plugin development lives under `src/plugins/dag/src_vN`.

## Current Version

- Current latest DAG version is `src_v3`.
- `src_v3` currently exposes a single section: `hit-test`.
- The `hit-test` section reuses the template v3 Three.js hit-testing behavior.

## Commands

```bash
./dialtone.sh dag install <src_vN>   # bun install for selected version UI
./dialtone.sh dag fmt <src_vN>       # go fmt for selected version
./dialtone.sh dag vet <src_vN>       # go vet for selected version
./dialtone.sh dag go-build <src_vN>  # go build for selected version
./dialtone.sh dag lint <src_vN>      # tsc --noEmit
./dialtone.sh dag format <src_vN>    # UI format check
./dialtone.sh dag build <src_vN>     # UI production build
./dialtone.sh dag serve <src_vN>     # Go server on :8080
./dialtone.sh dag ui-run <src_vN>    # Vite dev server (default :3000)
./dialtone.sh dag dev <src_vN>       # Vite + debug browser attach
./dialtone.sh dag test <src_vN>      # test_v2 suite -> TEST.md
./dialtone.sh dag smoke <src_vN>     # legacy smoke test where available
./dialtone.sh dag src --n <N>        # create a new src_vN
```
