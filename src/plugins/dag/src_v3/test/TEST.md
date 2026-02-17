# Dag Plugin src_v3 Test Report

**Generated at:** Mon, 16 Feb 2026 17:15:06 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `14.84s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 DuckDB Graph Query Validation | ✅ PASS | `63ms` |
| 02 Preflight (Go/UI) | ✅ PASS | `8.979s` |
| 03 DAG Table Section Validation | ✅ PASS | `2.187s` |
| 04 Menu/Nav Section Switch Validation | ✅ PASS | `205ms` |
| 05 User Story: Empty DAG Start + First Node | ✅ PASS | `255ms` |
| 06 User Story: Build Root IO | ✅ PASS | `308ms` |
| 07 User Story: Nest + Open Layer + Nested Build | ✅ PASS | `216ms` |
| 08 User Story: Rename + Close Layer + Camera History | ✅ PASS | `226ms` |
| 09 User Story: Deep Nested Build | ✅ PASS | `249ms` |
| 10 User Story: Deep Close Layer + Camera History | ✅ PASS | `218ms` |
| 11 User Story: Unlink + Relabel + Camera Readability | ✅ PASS | `264ms` |
| 12 Cleanup Verification | ✅ PASS | `6ms` |

## Step Logs

### 01 DuckDB Graph Query Validation

```text
result: PASS
duration: 63ms
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 DuckDB Graph Query Validation
[T+0000] [GRAPH] running: graph_edge_match_count
[T+0000] [GRAPH] running: shortest_path_hops_root_to_leaf
[T+0000] [GRAPH] running: rank_violation_count
[T+0000] [GRAPH] running: nested_nodes_for_n_mid_a
[T+0000] [GRAPH] running: input_nodes_for_n_leaf
[T+0000] [GRAPH] running: output_nodes_for_n_root
```

### 02 Preflight (Go/UI)

```text
result: PASS
duration: 8.979s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   02 Preflight (Go/UI)
[T+0000] >> [DAG] Fmt: src_v3
[T+0000] [2026-02-16T17:14:51.675-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/dag/src_v3/...]
[T+0000] >> [DAG] Vet: src_v3
[T+0000] [2026-02-16T17:14:52.110-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/dag/src_v3/...]
[T+0001] >> [DAG] Go Build: src_v3
[T+0001] [2026-02-16T17:14:52.698-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/dag/src_v3/...]
[T+0004] >> [DAG] Install: src_v3
[T+0004]    [DAG] duckdb already installed at /Users/dev/dialtone_dependencies/duckdb/bin/duckdb
[T+0004]    [DAG] Running version install hook: src/plugins/dag/src_v3/cmd/ops/install.go
[T+0004] [2026-02-16T17:14:55.524-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/ops/install.go]
[T+0004]    [DAG src_v3] Ensuring Go module dependency: github.com/marcboeker/go-duckdb
[T+0004] [2026-02-16T17:14:55.749-08:00 | INFO | go.go:RunGo:33] Running: go [mod download github.com/marcboeker/go-duckdb]
[T+0004] bun install v1.3.9 (cf6cdbbb)
[T+0004] 
[T+0004] + @types/three@0.182.0
[T+0004] + typescript@5.9.3
[T+0004] + vite@5.4.21
[T+0004] + three@0.182.0
[T+0004] 
[T+0004] 21 packages installed [138.00ms]
[T+0004] Saved lockfile
[T+0005] >> [DAG] Lint: src_v3
[T+0005] $ tsc --noEmit
[T+0006] >> [DAG] Format: src_v3
[T+0006] $ echo format-ok
[T+0006] format-ok
[T+0006] >> [DAG] Build: START for src_v3
[T+0006] >> [DAG] Installing UI dependencies in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0006] bun install v1.3.9 (cf6cdbbb)
[T+0007] 
[T+0007] + @types/three@0.182.0
[T+0007] + typescript@5.9.3
[T+0007] + vite@5.4.21
[T+0007] + three@0.182.0
[T+0007] 
[T+0007] 21 packages installed [160.00ms]
[T+0007] Saved lockfile
[T+0007] >> [DAG] Building UI in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0007] $ vite build
[T+0008] vite v5.4.21 building for production...
[T+0008] transforming...
[T+0008] ✓ 14 modules transformed.
[T+0008] rendering chunks...
[T+0009] computing gzip size...
[T+0009] dist/index.html                   3.36 kB │ gzip:   0.85 kB
[T+0009] dist/assets/index-C4XVJMyb.css    7.46 kB │ gzip:   2.27 kB
[T+0009] dist/assets/index-BFc8ONvj.js     2.11 kB │ gzip:   0.94 kB
[T+0009] dist/assets/index-DPgPk7H_.js    26.72 kB │ gzip:   7.14 kB
[T+0009] dist/assets/index-BzKeSYn1.js   524.23 kB │ gzip: 132.88 kB
[T+0009] 
[T+0009] (!) Some chunks are larger than 500 kB after minification. Consider:
[T+0009] - Using dynamic import() to code-split the application
[T+0009] - Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
[T+0009] - Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
[T+0009] ✓ built in 688ms
[T+0009] >> [DAG] Build: COMPLETE for src_v3
```

### 03 DAG Table Section Validation

```text
result: PASS
duration: 2.187s
section: dag-table
```

#### Runner Output

```text
[T+0009] [TEST] RUN   03 DAG Table Section Validation
[T+0009] >> [DAG] Serve: src_v3
[T+0009] [2026-02-16T17:15:00.663-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/main.go]
[T+0009] DAG Server starting on http://localhost:8080
[T+0009] [2026-02-16T17:15:00.815-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-50412 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080/#three]
```

![03 DAG Table Section Validation sequence](../screenshots/test_step_1_grid.png)

### 04 Menu/Nav Section Switch Validation

```text
result: PASS
duration: 205ms
section: three
```

#### Runner Output

```text
[T+0011] [TEST] RUN   04 Menu/Nav Section Switch Validation
```

![04 Menu/Nav Section Switch Validation sequence](../screenshots/test_step_menu_nav_grid.png)

### 05 User Story: Empty DAG Start + First Node

```text
result: PASS
duration: 255ms
section: three
```

#### Runner Output

```text
[T+0011] [TEST] RUN   05 User Story: Empty DAG Start + First Node
[T+0011] [THREE] story step1 description:
[T+0011] [THREE]   - In order to create a new node, the user taps Add.
[T+0011] [THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.
[T+0011] [THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.
```

![05 User Story: Empty DAG Start + First Node sequence](../screenshots/test_step_2_grid.png)

### 06 User Story: Build Root IO

```text
result: PASS
duration: 308ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   06 User Story: Build Root IO
[T+0012] [THREE] story step2 description:
[T+0012] [THREE]   - In order to add output, the user selects processor and taps Add.
[T+0012] [THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.
[T+0012] [THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.
[T+0012] [THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.
```

![06 User Story: Build Root IO sequence](../screenshots/test_step_3_grid.png)

### 07 User Story: Nest + Open Layer + Nested Build

```text
result: PASS
duration: 216ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   07 User Story: Nest + Open Layer + Nested Build
[T+0012] [THREE] story step3 description:
[T+0012] [THREE]   - In order to create a nested layer, the user selects processor and taps Nest.
[T+0012] [THREE]   - After opening the layer, user builds nested nodes using Add, then links them explicitly.
[T+0012] [THREE]   - Camera/layout expectation: nested layer anchors to parent x/z and elevates on +y; open-layer camera tracks that elevation.
```

![07 User Story: Nest + Open Layer + Nested Build sequence](../screenshots/test_step_4_grid.png)

### 08 User Story: Rename + Close Layer + Camera History

```text
result: PASS
duration: 226ms
section: three
```

#### Runner Output

```text
[T+0013] [TEST] RUN   08 User Story: Rename + Close Layer + Camera History
[T+0013] [THREE] story step4 description:
[T+0013] [THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.
[T+0013] [THREE]   - In order to close an opened layer, the user taps Back once to return to root.
[T+0013] [THREE]   - Camera expectation: layer close moves camera to the parent node and updates history to zero.
```

![08 User Story: Rename + Close Layer + Camera History sequence](../screenshots/test_step_5_grid.png)

### 09 User Story: Deep Nested Build

```text
result: PASS
duration: 249ms
section: three
```

#### Runner Output

```text
[T+0013] [TEST] RUN   09 User Story: Deep Nested Build
[T+0013] [THREE] story step5 description:
[T+0013] [THREE]   - In order to open an existing nested layer, user selects processor and taps Nest.
[T+0013] [THREE]   - In order to create second-level nested layer, user selects nested node and taps Nest.
[T+0013] [THREE]   - Camera/layout expectation: each deeper opened nested layer stacks higher on +y and camera y rises with depth.
```

![09 User Story: Deep Nested Build sequence](../screenshots/test_step_6_grid.png)

### 10 User Story: Deep Close Layer + Camera History

```text
result: PASS
duration: 218ms
section: three
```

#### Runner Output

```text
[T+0013] [TEST] RUN   10 User Story: Deep Close Layer + Camera History
[T+0013] [THREE] story step6 description:
[T+0013] [THREE]   - In order to close opened nested layers, user taps Back repeatedly.
[T+0013] [THREE]   - Each close action must reduce history depth and lower camera y as the stack unwinds.
[T+0013] [THREE]   - Final expectation: root layer visible with processor input/output context intact.
```

![10 User Story: Deep Close Layer + Camera History sequence](../screenshots/test_step_7_grid.png)

### 11 User Story: Unlink + Relabel + Camera Readability

```text
result: PASS
duration: 264ms
section: three
```

#### Runner Output

```text
[T+0014] [TEST] RUN   11 User Story: Unlink + Relabel + Camera Readability
[T+0014] [THREE] story step7 description:
[T+0014] [THREE]   - In order to remove edges, user selects output/input nodes and taps Unlink.
[T+0014] [THREE]   - User clears selections between unlink actions.
[T+0014] [THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.
```

![11 User Story: Unlink + Relabel + Camera Readability sequence](../screenshots/test_step_8_grid.png)

### 12 Cleanup Verification

```text
result: PASS
duration: 6ms
```

#### Runner Output

```text
[T+0014] [TEST] RUN   12 Cleanup Verification
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1_grid.png`
- `screenshots/test_step_1_pre.png`
- `screenshots/test_step_1.png`
- `screenshots/test_step_menu_nav_grid.png`
- `screenshots/test_step_menu_nav_pre.png`
- `screenshots/test_step_menu_nav.png`
- `screenshots/test_step_2_grid.png`
- `screenshots/test_step_2_pre.png`
- `screenshots/test_step_2.png`
- `screenshots/test_step_3_grid.png`
- `screenshots/test_step_3_pre.png`
- `screenshots/test_step_3.png`
- `screenshots/test_step_4_grid.png`
- `screenshots/test_step_4_pre.png`
- `screenshots/test_step_4.png`
- `screenshots/test_step_5_grid.png`
- `screenshots/test_step_5_pre.png`
- `screenshots/test_step_5.png`
- `screenshots/test_step_6_grid.png`
- `screenshots/test_step_6_pre.png`
- `screenshots/test_step_6.png`
- `screenshots/test_step_7_grid.png`
- `screenshots/test_step_7_pre.png`
- `screenshots/test_step_7.png`
- `screenshots/test_step_8_grid.png`
- `screenshots/test_step_8_pre.png`
- `screenshots/test_step_8.png`
