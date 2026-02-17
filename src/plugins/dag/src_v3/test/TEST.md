# Dag Plugin src_v3 Test Report

**Generated at:** Tue, 17 Feb 2026 15:56:45 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `40.145s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 DuckDB Graph Query Validation | ✅ PASS | `38ms` |
| 02 Preflight (Go/UI) | ✅ PASS | `9.321s` |
| 03 Startup: No Backend Menu -> Stage | ✅ PASS | `1.87s` |
| 04 Startup: Menu -> Stage Fresh Load | ✅ PASS | `1.197s` |
| 05 DAG Table Section Validation | ✅ PASS | `177ms` |
| 06 Menu/Nav Section Switch Validation | ✅ PASS | `472ms` |
| 07 User Story: Empty DAG Start + First Node | ✅ PASS | `2.274s` |
| 08 User Story: Build Root IO | ✅ PASS | `5.552s` |
| 09 User Story: Nest + Open Layer + Nested Build | ✅ PASS | `3.891s` |
| 10 User Story: Rename + Close Layer + Camera History | ✅ PASS | `2.889s` |
| 11 User Story: Deep Nested Build | ✅ PASS | `4.233s` |
| 12 User Story: Deep Close Layer + Camera History | ✅ PASS | `2.906s` |
| 13 User Story: Unlink + Relabel + Camera Readability | ✅ PASS | `3.913s` |
| 14 Cleanup Verification | ✅ PASS | `5ms` |

## Step Logs

### 01 DuckDB Graph Query Validation

```text
result: PASS
duration: 38ms
```

#### Step Story

Validated core DAG graph queries in DuckDB/duckpgq for edge count, shortest path, rank rules, and input/output nested-node derivations.

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
duration: 9.321s
```

#### Step Story

Ran preflight pipeline (`fmt`, `vet`, `go-build`, `install`, `lint`, `format`, `build`) to verify toolchain and UI build health before browser steps.

#### Runner Output

```text
[T+0000] [TEST] RUN   02 Preflight (Go/UI)
[T+0000] >> [DAG] Fmt: src_v3
[T+0000] [2026-02-17T15:56:05.952-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/dag/src_v3/...]
[T+0000] >> [DAG] Vet: src_v3
[T+0000] [2026-02-17T15:56:06.391-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/dag/src_v3/...]
[T+0001] >> [DAG] Go Build: src_v3
[T+0001] [2026-02-17T15:56:06.982-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/dag/src_v3/...]
[T+0004] >> [DAG] Install: src_v3
[T+0004]    [DAG] duckdb already installed at /Users/dev/dialtone_dependencies/duckdb/bin/duckdb
[T+0004]    [DAG] Running version install hook: src/plugins/dag/src_v3/cmd/ops/install.go
[T+0004] [2026-02-17T15:56:09.791-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/ops/install.go]
[T+0004]    [DAG src_v3] Ensuring Go module dependency: github.com/marcboeker/go-duckdb
[T+0004] [2026-02-17T15:56:10.016-08:00 | INFO | go.go:RunGo:33] Running: go [mod download github.com/marcboeker/go-duckdb]
[T+0004] bun install v1.3.9 (cf6cdbbb)
[T+0004] 
[T+0004] + @types/three@0.182.0
[T+0004] + typescript@5.9.3
[T+0004] + vite@5.4.21
[T+0004] + @xterm/xterm@6.0.0
[T+0004] + three@0.182.0
[T+0004] 
[T+0004] 22 packages installed [153.00ms]
[T+0004] Saved lockfile
[T+0005] >> [DAG] Lint: src_v3
[T+0005] $ tsc --noEmit
[T+0006] >> [DAG] Format: src_v3
[T+0006] $ echo format-ok
[T+0006] format-ok
[T+0006] >> [DAG] Build: START for src_v3
[T+0006] >> [DAG] Installing UI dependencies in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0006] bun install v1.3.9 (cf6cdbbb)
[T+0007] Saved lockfile
[T+0007] 
[T+0007] + @types/three@0.182.0
[T+0007] + typescript@5.9.3
[T+0007] + vite@5.4.21
[T+0007] + @xterm/xterm@6.0.0
[T+0007] + three@0.182.0
[T+0007] 
[T+0007] 22 packages installed [141.00ms]
[T+0007] >> [DAG] Building UI in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0007] $ vite build
[T+0008] vite v5.4.21 building for production...
[T+0008] transforming...
[T+0009] ✓ 16 modules transformed.
[T+0009] rendering chunks...
[T+0009] computing gzip size...
[T+0009] dist/index.html                   4.01 kB │ gzip:   0.98 kB
[T+0009] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0009] dist/assets/index-CXfBjiNH.css    8.65 kB │ gzip:   2.50 kB
[T+0009] dist/assets/index-Ba5WxkXE.js     3.23 kB │ gzip:   1.37 kB
[T+0009] dist/assets/index-Dnf_FwlV.js    13.53 kB │ gzip:   4.34 kB
[T+0009] dist/assets/index-DdBcr78Q.js   861.99 kB │ gzip: 218.27 kB
[T+0009] 
[T+0009] (!) Some chunks are larger than 500 kB after minification. Consider:
[T+0009] - Using dynamic import() to code-split the application
[T+0009] - Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
[T+0009] - Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
[T+0009] ✓ built in 941ms
[T+0009] >> [DAG] Build: COMPLETE for src_v3
```

### 03 Startup: No Backend Menu -> Stage

```text
result: PASS
duration: 1.87s
section: three
```

#### Step Story

With backend unavailable, loaded dev app, opened menu, navigated to Stage, and verified stage section becomes active and ready.

#### Runner Output

```text
[T+0009] [TEST] RUN   03 Startup: No Backend Menu -> Stage
[T+0009] Cleaning up stale process on port 8080 (PID: 58148)...
[T+0009] [2026-02-17T15:56:14.931-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-58512 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:3000/#three]
[T+0010] [WAIT] label=Toggle Global Menu detail=no-backend startup needs menu toggle
[T+0010] [CLICK] kind=aria target=Toggle Global Menu detail=open global menu without backend
[T+0010] [WAIT] label=Navigate Stage detail=no-backend startup needs stage nav button
[T+0011] [CLICK] kind=aria target=Navigate Stage detail=switch to stage without backend
[T+0011] [WAIT] label=Three Canvas detail=stage canvas should exist without backend
[T+0011] [WAIT] label=Three Section detail=stage section should be active without backend
[T+0011] [WAIT] label=Three Canvas detail=stage should report ready without backend
```

![03 Startup: No Backend Menu -> Stage sequence](../screenshots/test_step_no_backend_menu_stage_grid.png)

### 04 Startup: Menu -> Stage Fresh Load

```text
result: PASS
duration: 1.197s
section: three
```

#### Step Story

Fresh app startup opened menu immediately, used Navigate Stage, and verified the stage section becomes active and ready without requiring table readiness.

#### Runner Output

```text
[T+0011] [TEST] RUN   04 Startup: Menu -> Stage Fresh Load
[T+0011] >> [DAG] Serve: src_v3
[T+0011] [2026-02-17T15:56:17.318-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/main.go]
[T+0011] DAG Server starting on http://localhost:8080
[T+0011] [WAIT] label=Toggle Global Menu detail=fresh startup needs menu toggle
[T+0012] [CLICK] kind=aria target=Toggle Global Menu detail=open global menu from fresh startup
[T+0012] [WAIT] label=Navigate Stage detail=fresh startup needs stage nav button
[T+0012] [CLICK] kind=aria target=Navigate Stage detail=switch to stage from menu
[T+0012] [WAIT] label=Three Canvas detail=stage canvas should exist after menu nav
[T+0012] [WAIT] label=Three Section detail=stage section should be active
[T+0012] [WAIT] label=Three Canvas detail=stage should report ready
```

![04 Startup: Menu -> Stage Fresh Load sequence](../screenshots/test_step_startup_menu_stage_grid.png)

### 05 DAG Table Section Validation

```text
result: PASS
duration: 177ms
section: dag-table
```

#### Step Story

Loaded the DAG table, waited for `data-ready=true`, validated API parity and row status content, then captured pre/post table screenshots.

#### Runner Output

```text
[T+0012] [TEST] RUN   05 DAG Table Section Validation
[T+0012] [WAIT] label=DAG Table detail=need table element for validation
[T+0012] [WAIT] label=DAG Table detail=wait for table ready flag
```

![05 DAG Table Section Validation sequence](../screenshots/test_step_1_grid.png)

### 06 Menu/Nav Section Switch Validation

```text
result: PASS
duration: 472ms
section: three
```

#### Step Story

Opened global menu from table, navigated to stage through menu action, and verified the stage canvas becomes ready after section switch.

#### Runner Output

```text
[T+0012] [TEST] RUN   06 Menu/Nav Section Switch Validation
[T+0012] [WAIT] label=Toggle Global Menu detail=need menu toggle
[T+0012] [WAIT] label=DAG Table detail=need table visible
[T+0012] [WAIT] label=DAG Table detail=wait for table ready
[T+0012] [CLICK] kind=aria target=Toggle Global Menu detail=open global menu
[T+0013] [WAIT] label=Navigate Stage detail=need stage menu button
[T+0013] [CLICK] kind=aria target=Navigate Stage detail=switch section to stage
[T+0013] [WAIT] label=Three Canvas detail=confirm stage visible after nav
[T+0013] [WAIT] label=Three Canvas detail=wait for stage ready after nav
```

![06 Menu/Nav Section Switch Validation sequence](../screenshots/test_step_menu_nav_grid.png)

### 07 User Story: Empty DAG Start + First Node

```text
result: PASS
duration: 2.274s
section: three
```

#### Step Story

Loaded the stage controls, added the first node, cycled camera modes (top/side/iso), and reselected the new node to verify interaction readiness.

#### Runner Output

```text
[T+0013] [TEST] RUN   07 User Story: Empty DAG Start + First Node
[T+0013] [THREE] story step1 description:
[T+0013] [THREE]   - In order to create a new node, the user taps Add.
[T+0013] [THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.
[T+0013] [THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.
[T+0013] [WAIT] label=Three Canvas detail=need stage canvas before interactions
[T+0013] [WAIT] label=Three Canvas detail=wait for stage ready flag
[T+0013] [WAIT] label=DAG Mode detail=need mode button
[T+0013] [WAIT] label=DAG Thumb 1 detail=need thumb 1
[T+0013] [WAIT] label=DAG Thumb 2 detail=need thumb 2
[T+0013] [WAIT] label=DAG Thumb 3 detail=need thumb 3
[T+0013] [WAIT] label=DAG Label Input detail=need rename input
[T+0013] [CLICK] kind=action target=add detail=mode=graph
[T+0013] [CLICK] kind=mode target=DAG Mode detail=target=camera
[T+0013] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0014] [CLICK] kind=mode target=DAG Mode detail=target=camera
[T+0014] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0014] [CLICK] kind=action target=camera_top detail=mode=camera
[T+0015] [CLICK] kind=action target=camera_side detail=mode=camera
[T+0015] [CLICK] kind=action target=camera_iso detail=mode=camera
[T+0015] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0015] [CLICK] kind=step_done target=story_step_1 detail=ok
```

![07 User Story: Empty DAG Start + First Node sequence](../screenshots/test_step_2_grid.png)

### 08 User Story: Build Root IO

```text
result: PASS
duration: 5.552s
section: three
```

#### Step Story

Built root IO by creating output and input nodes around processor, linked both directions via selection pair semantics, and validated back/clear interaction flow.

#### Runner Output

```text
[T+0015] [TEST] RUN   08 User Story: Build Root IO
[T+0015] [THREE] story step2 description:
[T+0015] [THREE]   - In order to add output, the user selects processor and taps Add.
[T+0015] [THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.
[T+0015] [THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.
[T+0015] [THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.
[T+0015] [WAIT] label=Three Canvas detail=need canvas before story step2 actions
[T+0016] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0016] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0016] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0016] [CLICK] kind=action target=add detail=mode=graph
[T+0017] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0017] [CLICK] kind=node target=n_user_1 detail=x=29,y=241
[T+0017] [CLICK] kind=node target=n_user_2 detail=x=417,y=501
[T+0018] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0018] [CLICK] kind=canvas target=Three Canvas detail=clear-selection;x=8,y=8
[T+0018] [CLICK] kind=action target=add detail=mode=graph
[T+0019] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0019] [CLICK] kind=node target=n_user_3 detail=x=195,y=352
[T+0019] [CLICK] kind=node target=n_user_1 detail=x=50,y=449
[T+0020] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0020] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0020] [CLICK] kind=node target=n_user_3 detail=x=-111,y=314
[T+0021] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0021] [CLICK] kind=action target=back detail=mode=graph
[T+0021] [CLICK] kind=step_done target=story_step_2 detail=ok
```

![08 User Story: Build Root IO sequence](../screenshots/test_step_3_grid.png)

### 09 User Story: Nest + Open Layer + Nested Build

```text
result: PASS
duration: 3.891s
section: three
```

#### Step Story

Opened processor nested layer, created two nested nodes, linked them, and preserved selection context inside the nested layer.

#### Runner Output

```text
[T+0021] [TEST] RUN   09 User Story: Nest + Open Layer + Nested Build
[T+0021] [THREE] story step3 description:
[T+0021] [THREE]   - In order to create/open a nested layer, the user selects processor, switches to Layer mode, and taps Open Layer.
[T+0021] [THREE]   - After opening the layer, user builds nested nodes using Add, then links them explicitly.
[T+0021] [THREE]   - Camera/layout expectation: nested layer anchors to parent x/z and elevates on +y; open-layer camera tracks that elevation.
[T+0021] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0021] [CLICK] kind=mode target=DAG Mode detail=target=layer
[T+0022] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0022] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0022] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0022] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0022] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0023] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0023] [CLICK] kind=action target=add detail=mode=graph
[T+0023] [CLICK] kind=action target=add detail=mode=graph
[T+0024] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0024] [CLICK] kind=node target=n_user_4 detail=x=29,y=241
[T+0024] [CLICK] kind=node target=n_user_5 detail=x=417,y=501
[T+0025] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0025] [CLICK] kind=node target=n_user_4 detail=x=195,y=352
[T+0025] [CLICK] kind=step_done target=story_step_3 detail=ok
```

![09 User Story: Nest + Open Layer + Nested Build sequence](../screenshots/test_step_4_grid.png)

### 10 User Story: Rename + Close Layer + Camera History

```text
result: PASS
duration: 2.889s
section: three
```

#### Step Story

Renamed nested node, backed out to parent layer, closed nested layer from parent context, and renamed processor in root context.

#### Runner Output

```text
[T+0025] [TEST] RUN   10 User Story: Rename + Close Layer + Camera History
[T+0025] [THREE] story step4 description:
[T+0025] [THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.
[T+0025] [THREE]   - In order to close an opened layer, the user switches to Layer mode and taps Close Layer.
[T+0025] [THREE]   - Camera expectation: layer close moves camera to the parent node and updates history to zero.
[T+0025] [CLICK] kind=node target=n_user_4 detail=x=195,y=352
[T+0025] [CLICK] kind=rename_submit target=DAG Rename detail=Nested Input
[T+0026] [CLICK] kind=aria target=DAG Rename detail=submit rename
[T+0026] [CLICK] kind=action target=back detail=mode=graph
[T+0026] [CLICK] kind=mode target=DAG Mode detail=target=layer
[T+0026] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0027] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0027] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0027] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0027] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0027] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0028] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0028] [CLICK] kind=rename_submit target=DAG Rename detail=Processor
[T+0028] [CLICK] kind=aria target=DAG Rename detail=submit rename
[T+0028] [CLICK] kind=step_done target=story_step_4 detail=ok
```

![10 User Story: Rename + Close Layer + Camera History sequence](../screenshots/test_step_5_grid.png)

### 11 User Story: Deep Nested Build

```text
result: PASS
duration: 4.233s
section: three
```

#### Step Story

Re-opened processor nested layer, opened second-level nested layer, created deeper nodes, and linked them to validate multi-depth DAG interaction.

#### Runner Output

```text
[T+0028] [TEST] RUN   11 User Story: Deep Nested Build
[T+0028] [THREE] story step5 description:
[T+0028] [THREE]   - In order to open an existing nested layer, user selects processor and taps Open Layer in Layer mode.
[T+0028] [THREE]   - In order to create second-level nested layer, user selects nested node and taps Open Layer in Layer mode.
[T+0028] [THREE]   - Camera/layout expectation: each deeper opened nested layer stacks higher on +y and camera y rises with depth.
[T+0028] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0028] [CLICK] kind=mode target=DAG Mode detail=target=layer
[T+0029] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0029] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0029] [CLICK] kind=node target=n_user_5 detail=x=247,y=419
[T+0030] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0030] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0030] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0030] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0030] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0031] [CLICK] kind=action target=add detail=mode=graph
[T+0031] [CLICK] kind=action target=add detail=mode=graph
[T+0031] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0032] [CLICK] kind=node target=n_user_6 detail=x=29,y=241
[T+0032] [CLICK] kind=node target=n_user_7 detail=x=417,y=501
[T+0032] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0032] [CLICK] kind=step_done target=story_step_5 detail=ok
```

![11 User Story: Deep Nested Build sequence](../screenshots/test_step_6_grid.png)

### 12 User Story: Deep Close Layer + Camera History

```text
result: PASS
duration: 2.906s
section: three
```

#### Step Story

Closed deep nested layers in parent-first flow (`back` then `open/close`), returned to root processor context, and verified unwind behavior.

#### Runner Output

```text
[T+0033] [TEST] RUN   12 User Story: Deep Close Layer + Camera History
[T+0033] [THREE] story step6 description:
[T+0033] [THREE]   - In order to close opened nested layers, user stays in Layer mode and taps Close Layer repeatedly.
[T+0033] [THREE]   - Each close action must reduce history depth and lower camera y as the stack unwinds.
[T+0033] [THREE]   - Final expectation: root layer visible with processor input/output context intact.
[T+0033] [CLICK] kind=action target=back detail=mode=graph
[T+0033] [CLICK] kind=mode target=DAG Mode detail=target=layer
[T+0033] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0033] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0033] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0034] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0034] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0034] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0034] [CLICK] kind=action target=back detail=mode=graph
[T+0034] [CLICK] kind=mode target=DAG Mode detail=target=layer
[T+0035] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0035] [CLICK] kind=action target=open_or_close_layer detail=mode=layer; clicking open/close to change layer
[T+0035] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0035] [CLICK] kind=step_done target=story_step_6 detail=ok
```

![12 User Story: Deep Close Layer + Camera History sequence](../screenshots/test_step_7_grid.png)

### 13 User Story: Unlink + Relabel + Camera Readability

```text
result: PASS
duration: 3.913s
section: three
```

#### Step Story

Unlinked input->processor and processor->output edges using context link/unlink action, then relabeled processor to final state.

#### Runner Output

```text
[T+0036] [TEST] RUN   13 User Story: Unlink + Relabel + Camera Readability
[T+0036] [THREE] story step7 description:
[T+0036] [THREE]   - In order to remove edges, user selects output/input nodes and taps the context Link/Unlink button.
[T+0036] [THREE]   - User clears selections between unlink actions.
[T+0036] [THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.
[T+0036] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0036] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0036] [CLICK] kind=mode target=DAG Mode detail=target=graph
[T+0036] [CLICK] kind=aria target=DAG Mode detail=switch mode
[T+0036] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0037] [CLICK] kind=node target=n_user_3 detail=x=-111,y=314
[T+0037] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0037] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0038] [CLICK] kind=action target=clear_picks detail=mode=graph
[T+0038] [CLICK] kind=node target=n_user_1 detail=x=195,y=352
[T+0038] [CLICK] kind=node target=n_user_2 detail=x=50,y=449
[T+0039] [CLICK] kind=action target=link_or_unlink detail=mode=graph
[T+0039] [CLICK] kind=node target=n_user_1 detail=x=29,y=241
[T+0039] [CLICK] kind=rename_submit target=DAG Rename detail=Processor Final
[T+0039] [CLICK] kind=aria target=DAG Rename detail=submit rename
[T+0039] [CLICK] kind=step_done target=story_step_7 detail=ok
```

![13 User Story: Unlink + Relabel + Camera Readability sequence](../screenshots/test_step_8_grid.png)

### 14 Cleanup Verification

```text
result: PASS
duration: 5ms
```

#### Step Story

Closed shared test server/browser resources and left attach-mode preview session running as configured.

#### Runner Output

```text
[T+0040] [TEST] RUN   14 Cleanup Verification
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_no_backend_menu_stage_grid.png`
- `screenshots/test_step_no_backend_menu_stage_pre.png`
- `screenshots/test_step_no_backend_menu_stage.png`
- `screenshots/test_step_startup_menu_stage_grid.png`
- `screenshots/test_step_startup_menu_stage_pre.png`
- `screenshots/test_step_startup_menu_stage.png`
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
