# Dag Plugin src_v3 Test Report

**Generated at:** Mon, 16 Feb 2026 13:32:05 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `14.956s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 DuckDB Graph Query Validation | ✅ PASS | `50ms` |
| 02 Preflight (Go/UI) | ✅ PASS | `8.941s` |
| 03 DAG Table Section Validation | ✅ PASS | `1.131s` |
| 04 User Story: Empty DAG Start + First Node | ✅ PASS | `638ms` |
| 05 User Story: Build Root IO | ✅ PASS | `491ms` |
| 06 User Story: Nest + Open Layer + Nested Build | ✅ PASS | `510ms` |
| 07 User Story: Rename + Close Layer + Camera History | ✅ PASS | `501ms` |
| 08 User Story: Deep Nested Build | ✅ PASS | `516ms` |
| 09 User Story: Deep Close Layer + Camera History | ✅ PASS | `505ms` |
| 10 User Story: Unlink + Relabel + Camera Readability | ✅ PASS | `524ms` |
| 11 Cleanup Verification | ✅ PASS | `0s` |

## Step Logs

### 01 DuckDB Graph Query Validation

```text
result: PASS
duration: 50ms
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
duration: 8.941s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   02 Preflight (Go/UI)
[T+0000] >> [DAG] Fmt: src_v3
[T+0000] [2026-02-16T13:31:51.235-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/dag/src_v3/...]
[T+0000] >> [DAG] Vet: src_v3
[T+0000] [2026-02-16T13:31:51.676-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/dag/src_v3/...]
[T+0001] >> [DAG] Go Build: src_v3
[T+0001] [2026-02-16T13:31:52.257-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/dag/src_v3/...]
[T+0004] >> [DAG] Install: src_v3
[T+0004]    [DAG] duckdb already installed at /Users/dev/dialtone_dependencies/duckdb/bin/duckdb
[T+0004]    [DAG] Running version install hook: src/plugins/dag/src_v3/cmd/ops/install.go
[T+0004] [2026-02-16T13:31:55.087-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/ops/install.go]
[T+0004]    [DAG src_v3] Ensuring Go module dependency: github.com/marcboeker/go-duckdb
[T+0004] [2026-02-16T13:31:55.308-08:00 | INFO | go.go:RunGo:33] Running: go [mod download github.com/marcboeker/go-duckdb]
[T+0004] bun install v1.3.9 (cf6cdbbb)
[T+0004] 
[T+0004] + @types/three@0.182.0
[T+0004] + typescript@5.9.3
[T+0004] + vite@5.4.21
[T+0004] + three@0.182.0
[T+0004] 
[T+0004] 21 packages installed [160.00ms]
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
[T+0007] 21 packages installed [162.00ms]
[T+0007] Saved lockfile
[T+0007] >> [DAG] Building UI in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0007] $ vite build
[T+0008] vite v5.4.21 building for production...
[T+0008] transforming...
[T+0008] ✓ 14 modules transformed.
[T+0008] rendering chunks...
[T+0008] computing gzip size...
[T+0008] dist/index.html                   3.36 kB │ gzip:   0.85 kB
[T+0008] dist/assets/index-oDEfVqVm.css    7.25 kB │ gzip:   2.21 kB
[T+0008] dist/assets/index-BFc8ONvj.js     2.11 kB │ gzip:   0.94 kB
[T+0008] dist/assets/index-C7_qROU5.js    25.52 kB │ gzip:   6.90 kB
[T+0008] dist/assets/index-Dc6JYZKO.js   522.82 kB │ gzip: 132.55 kB
[T+0008] 
[T+0008] (!) Some chunks are larger than 500 kB after minification. Consider:
[T+0008] - Using dynamic import() to code-split the application
[T+0008] - Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
[T+0008] - Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
[T+0008] ✓ built in 665ms
[T+0008] >> [DAG] Build: COMPLETE for src_v3
```

### 03 DAG Table Section Validation

```text
result: PASS
duration: 1.131s
section: dag-table
```

#### Runner Output

```text
[T+0008] [TEST] RUN   03 DAG Table Section Validation
[T+0009] Cleaning up stale process on port 8080 (PID: 37244)...
[T+0009] >> [DAG] Serve: src_v3
[T+0009] [2026-02-16T13:32:00.199-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/main.go]
[T+0009] DAG Server starting on http://localhost:8080
```

![03 DAG Table Section Validation sequence](../screenshots/test_step_1_grid.png)

### 04 User Story: Empty DAG Start + First Node

```text
result: PASS
duration: 638ms
section: three
```

#### Runner Output

```text
[T+0010] [TEST] RUN   04 User Story: Empty DAG Start + First Node
[T+0010] [THREE] story step1 description:
[T+0010] [THREE]   - In order to create a new node, the user taps Add.
[T+0010] [THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.
[T+0010] [THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.
```

![04 User Story: Empty DAG Start + First Node sequence](../screenshots/test_step_2_grid.png)

### 05 User Story: Build Root IO

```text
result: PASS
duration: 491ms
section: three
```

#### Runner Output

```text
[T+0010] [TEST] RUN   05 User Story: Build Root IO
[T+0010] [THREE] story step2 description:
[T+0010] [THREE]   - In order to add output, the user selects processor and taps Add.
[T+0010] [THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.
[T+0010] [THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.
[T+0010] [THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.
```

![05 User Story: Build Root IO sequence](../screenshots/test_step_3_grid.png)

### 06 User Story: Nest + Open Layer + Nested Build

```text
result: PASS
duration: 510ms
section: three
```

#### Runner Output

```text
[T+0011] [TEST] RUN   06 User Story: Nest + Open Layer + Nested Build
[T+0011] [THREE] story step3 description:
[T+0011] [THREE]   - In order to create a nested layer, the user selects processor and taps Nest.
[T+0011] [THREE]   - After opening the layer, user builds nested nodes using Add, then links them explicitly.
[T+0011] [THREE]   - Camera/layout expectation: nested layer anchors to parent x/z and elevates on +y; open-layer camera tracks that elevation.
```

![06 User Story: Nest + Open Layer + Nested Build sequence](../screenshots/test_step_4_grid.png)

### 07 User Story: Rename + Close Layer + Camera History

```text
result: PASS
duration: 501ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   07 User Story: Rename + Close Layer + Camera History
[T+0012] [THREE] story step4 description:
[T+0012] [THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.
[T+0012] [THREE]   - In order to close an opened layer, the user taps Back once to return to root.
[T+0012] [THREE]   - Camera expectation: layer close moves camera to the parent node and updates history to zero.
```

![07 User Story: Rename + Close Layer + Camera History sequence](../screenshots/test_step_5_grid.png)

### 08 User Story: Deep Nested Build

```text
result: PASS
duration: 516ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   08 User Story: Deep Nested Build
[T+0012] [THREE] story step5 description:
[T+0012] [THREE]   - In order to open an existing nested layer, user selects processor and taps Nest.
[T+0012] [THREE]   - In order to create second-level nested layer, user selects nested node and taps Nest.
[T+0012] [THREE]   - Camera/layout expectation: each deeper opened nested layer stacks higher on +y and camera y rises with depth.
```

![08 User Story: Deep Nested Build sequence](../screenshots/test_step_6_grid.png)

### 09 User Story: Deep Close Layer + Camera History

```text
result: PASS
duration: 505ms
section: three
```

#### Runner Output

```text
[T+0013] [TEST] RUN   09 User Story: Deep Close Layer + Camera History
[T+0013] [THREE] story step6 description:
[T+0013] [THREE]   - In order to close opened nested layers, user taps Back repeatedly.
[T+0013] [THREE]   - Each close action must reduce history depth and lower camera y as the stack unwinds.
[T+0013] [THREE]   - Final expectation: root layer visible with processor input/output context intact.
```

![09 User Story: Deep Close Layer + Camera History sequence](../screenshots/test_step_7_grid.png)

### 10 User Story: Unlink + Relabel + Camera Readability

```text
result: PASS
duration: 524ms
section: three
```

#### Runner Output

```text
[T+0014] [TEST] RUN   10 User Story: Unlink + Relabel + Camera Readability
[T+0014] [THREE] story step7 description:
[T+0014] [THREE]   - In order to remove edges, user selects output/input nodes and taps Unlink.
[T+0014] [THREE]   - User clears selections between unlink actions.
[T+0014] [THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.
```

![10 User Story: Unlink + Relabel + Camera Readability sequence](../screenshots/test_step_8_grid.png)

### 11 Cleanup Verification

```text
result: PASS
duration: 0s
```

#### Runner Output

```text
[T+0014] [TEST] RUN   11 Cleanup Verification
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1_grid.png`
- `screenshots/test_step_1_pre.png`
- `screenshots/test_step_1.png`
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
