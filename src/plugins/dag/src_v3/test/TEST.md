# Template Plugin v3 Test Report

**Generated at:** Sun, 15 Feb 2026 10:47:27 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `13.123s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 DuckDB Graph Query Validation | ✅ PASS | `115ms` |
| 02 Preflight (Go/UI) | ✅ PASS | `9.959s` |
| 03 DAG Table Section Validation | ✅ PASS | `2.75s` |
| 04 Three Section Validation | ✅ PASS | `291ms` |
| 05 Cleanup Verification | ✅ PASS | `8ms` |

## Step Logs

### 01 DuckDB Graph Query Validation

```text
result: PASS
duration: 115ms
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 DuckDB Graph Query Validation
```

### 02 Preflight (Go/UI)

```text
result: PASS
duration: 9.959s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   02 Preflight (Go/UI)
[T+0000] >> [DAG] Fmt: src_v3
[T+0000] [2026-02-15T10:47:15.341-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/dag/src_v3/...]
[T+0000] >> [DAG] Vet: src_v3
[T+0000] [2026-02-15T10:47:15.801-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/dag/src_v3/...]
[T+0001] >> [DAG] Go Build: src_v3
[T+0001] [2026-02-15T10:47:16.429-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/dag/src_v3/...]
[T+0005] >> [DAG] Install: src_v3
[T+0005]    [DAG] duckdb already installed at /Users/dev/dialtone_dependencies/duckdb/bin/duckdb
[T+0005] bun install v1.3.9 (cf6cdbbb)
[T+0005] Saved lockfile
[T+0005] 
[T+0005] + @types/three@0.182.0
[T+0005] + typescript@5.9.3
[T+0005] + vite@5.4.21
[T+0005] + three@0.182.0
[T+0005] 
[T+0005] 21 packages installed [197.00ms]
[T+0005] >> [DAG] Lint: src_v3
[T+0005] $ tsc --noEmit
[T+0007] >> [DAG] Format: src_v3
[T+0007] $ echo format-ok
[T+0007] format-ok
[T+0007] >> [DAG] Build: START for src_v3
[T+0007] >> [DAG] Installing UI dependencies in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0007] bun install v1.3.9 (cf6cdbbb)
[T+0007] Saved lockfile
[T+0007] 
[T+0007] + @types/three@0.182.0
[T+0007] + typescript@5.9.3
[T+0007] + vite@5.4.21
[T+0007] + three@0.182.0
[T+0007] 
[T+0007] 21 packages installed [169.00ms]
[T+0007] >> [DAG] Building UI in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0008] $ vite build
[T+0009] vite v5.4.21 building for production...
[T+0009] transforming...
[T+0009] ✓ 13 modules transformed.
[T+0009] rendering chunks...
[T+0010] computing gzip size...
[T+0010] dist/index.html                   1.04 kB │ gzip:   0.47 kB
[T+0010] dist/assets/index-DYW-3Y5m.css    3.39 kB │ gzip:   1.22 kB
[T+0010] dist/assets/index-mv4QQ1Rd.js     2.01 kB │ gzip:   0.93 kB
[T+0010] dist/assets/index-Ct2d_kCW.js     6.78 kB │ gzip:   2.40 kB
[T+0010] dist/assets/index-DNUqOQTF.js   492.97 kB │ gzip: 125.16 kB
[T+0010] ✓ built in 729ms
[T+0010] >> [DAG] Build: COMPLETE for src_v3
```

### 03 DAG Table Section Validation

```text
result: PASS
duration: 2.75s
section: dag-table
```

#### Runner Output

```text
[T+0010] [TEST] RUN   03 DAG Table Section Validation
[T+0010] >> [DAG] Serve: src_v3
[T+0010] [2026-02-15T10:47:25.391-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/main.go]
[T+0010] DAG Server starting on http://localhost:8080
[T+0010] [2026-02-15T10:47:25.729-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-50230 --new-window --dialtone-origin=true --dialtone-role=test --headless=new]
[T+0012] [BROWSER] [log] [SectionManager] NAVIGATING TO #dag-table
[T+0012] [BROWSER] [log] [SectionManager] LOADING #dag-table
[T+0012] [BROWSER] [log] [SectionManager] NAVIGATING TO #dag-table
[T+0012] [BROWSER] [log] [SectionManager] LOADED #dag-table
[T+0012] [BROWSER] [log] [SectionManager] START #dag-table
[T+0012] [BROWSER] [log] [SectionManager] NAVIGATE TO #dag-table
[T+0012] [BROWSER] [log] [SectionManager] NAVIGATE TO #dag-table
[T+0012] [BROWSER] [log] [SectionManager] RESUME #dag-table
[T+0012] [BROWSER] [log] [SectionManager] RESUME #dag-table
[T+0012] [BROWSER] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #dag-table
[T+0012] [log] [SectionManager] LOADING #dag-table
[T+0012] [log] [SectionManager] NAVIGATING TO #dag-table
[T+0012] [log] [SectionManager] LOADED #dag-table
[T+0012] [log] [SectionManager] START #dag-table
[T+0012] [log] [SectionManager] NAVIGATE TO #dag-table
[T+0012] [log] [SectionManager] NAVIGATE TO #dag-table
[T+0012] [log] [SectionManager] RESUME #dag-table
[T+0012] [log] [SectionManager] RESUME #dag-table
```

![03 DAG Table Section Validation](../screenshots/test_step_1.png)

### 04 Three Section Validation

```text
result: PASS
duration: 291ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   04 Three Section Validation
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #three
[T+0012] [log] [SectionManager] LOADING #three
[T+0012] [log] [SectionManager] NAVIGATING TO #three
[T+0012] [log] [SectionManager] LOADED #three
[T+0012] [log] [SectionManager] START #three
[T+0012] [log] [SectionManager] NAVIGATE TO #three
[T+0012] [log] [SectionManager] NAVIGATE TO #three
[T+0012] [log] [SectionManager] RESUME #three
[T+0012] [log] [SectionManager] RESUME #three
[T+0013] [log] [Three #three] touch cube: cube_left
```

![04 Three Section Validation](../screenshots/test_step_2.png)

### 05 Cleanup Verification

```text
result: PASS
duration: 8ms
```

#### Runner Output

```text
[T+0013] [TEST] RUN   05 Cleanup Verification
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1.png`
- `screenshots/test_step_2.png`
