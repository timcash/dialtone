# Template Plugin v3 Test Report

**Generated at:** Sun, 15 Feb 2026 08:56:27 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `9.577s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `7.413s` |
| 02 Hit-Test Section Validation | ✅ PASS | `2.158s` |
| 03 Cleanup Verification | ✅ PASS | `5ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 7.413s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] >> [DAG] Fmt: src_v3
[T+0000] [2026-02-15T08:56:18.237-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/dag/src_v3/...]
[T+0000] >> [DAG] Vet: src_v3
[T+0000] [2026-02-15T08:56:18.674-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/dag/src_v3/...]
[T+0001] >> [DAG] Go Build: src_v3
[T+0001] [2026-02-15T08:56:19.226-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/dag/src_v3/...]
[T+0002] >> [DAG] Install: src_v3
[T+0003] bun install v1.3.9 (cf6cdbbb)
[T+0003] Saved lockfile
[T+0003] 
[T+0003] + @types/three@0.182.0
[T+0003] + typescript@5.9.3
[T+0003] + vite@5.4.21
[T+0003] + three@0.182.0
[T+0003] 
[T+0003] 21 packages installed [162.00ms]
[T+0003] >> [DAG] Lint: src_v3
[T+0003] $ tsc --noEmit
[T+0004] >> [DAG] Format: src_v3
[T+0004] $ echo format-ok
[T+0004] format-ok
[T+0005] >> [DAG] Build: START for src_v3
[T+0005] >> [DAG] Installing UI dependencies in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0005] bun install v1.3.9 (cf6cdbbb)
[T+0005] 
[T+0005] + @types/three@0.182.0
[T+0005] + typescript@5.9.3
[T+0005] + vite@5.4.21
[T+0005] + three@0.182.0
[T+0005] 
[T+0005] 21 packages installed [135.00ms]
[T+0005] Saved lockfile
[T+0005] >> [DAG] Building UI in /Users/dev/code/dialtone/src/plugins/dag/src_v3/ui...
[T+0005] $ vite build
[T+0006] vite v5.4.21 building for production...
[T+0006] transforming...
[T+0007] ✓ 12 modules transformed.
[T+0007] rendering chunks...
[T+0007] computing gzip size...
[T+0007] dist/index.html                   0.72 kB │ gzip:   0.39 kB
[T+0007] dist/assets/index-PKZAucp0.css    2.91 kB │ gzip:   1.10 kB
[T+0007] dist/assets/index-fnp8iTlC.js     6.34 kB │ gzip:   2.32 kB
[T+0007] dist/assets/index-CSjRd_R3.js   492.98 kB │ gzip: 125.17 kB
[T+0007] ✓ built in 658ms
[T+0007] >> [DAG] Build: COMPLETE for src_v3
```

### 02 Hit-Test Section Validation

```text
result: PASS
duration: 2.158s
section: hit-test
```

#### Runner Output

```text
[T+0007] [TEST] RUN   02 Hit-Test Section Validation
[T+0007] >> [DAG] Serve: src_v3
[T+0007] [2026-02-15T08:56:25.628-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/dag/src_v3/cmd/main.go]
[T+0007] DAG Server starting on http://localhost:8080
[T+0007] [2026-02-15T08:56:25.785-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-63664 --new-window --dialtone-origin=true --dialtone-role=test --headless=new]
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATING TO #hit-test
[T+0009] [BROWSER] [log] [SectionManager] LOADING #hit-test
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATING TO #hit-test
[T+0009] [BROWSER] [log] [SectionManager] LOADED #hit-test
[T+0009] [BROWSER] [log] [SectionManager] START #hit-test
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATE TO #hit-test
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATE TO #hit-test
[T+0009] [BROWSER] [error] [PROOFOFLIFE] Intentional Browser Test Error
[T+0009] [BROWSER] [log] [SectionManager] RESUME #hit-test
[T+0009] [BROWSER] [log] [SectionManager] RESUME #hit-test
[T+0009] [BROWSER] [log] [HitTest #hit-test] touch cube: cube_left
```

#### Browser Logs

```text
[T+0009] [log] [SectionManager] NAVIGATING TO #hit-test
[T+0009] [log] [SectionManager] LOADING #hit-test
[T+0009] [log] [SectionManager] NAVIGATING TO #hit-test
[T+0009] [log] [SectionManager] LOADED #hit-test
[T+0009] [log] [SectionManager] START #hit-test
[T+0009] [log] [SectionManager] NAVIGATE TO #hit-test
[T+0009] [log] [SectionManager] NAVIGATE TO #hit-test
[T+0009] [log] [SectionManager] RESUME #hit-test
[T+0009] [log] [SectionManager] RESUME #hit-test
[T+0009] [log] [HitTest #hit-test] touch cube: cube_left
```

![02 Hit-Test Section Validation](../screenshots/test_step_1.png)

### 03 Cleanup Verification

```text
result: PASS
duration: 5ms
```

#### Runner Output

```text
[T+0009] [TEST] RUN   03 Cleanup Verification
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1.png`
