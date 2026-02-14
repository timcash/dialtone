# Template Plugin v3 Test Report

**Generated at:** Sat, 14 Feb 2026 10:29:03 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `14.847s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `6.933s` |
| 02 Go Run | ✅ PASS | `627ms` |
| 03 UI Run | ✅ PASS | `756ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `1.437s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `784ms` |
| 06 Hero Section Validation | ✅ PASS | `88ms` |
| 07 Docs Section Validation | ✅ PASS | `355ms` |
| 08 Table Section Validation | ✅ PASS | `447ms` |
| 09 Three Section Validation | ✅ PASS | `429ms` |
| 10 Xterm Section Validation | ✅ PASS | `389ms` |
| 11 Video Section Validation | ✅ PASS | `492ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.905s` |
| 13 Cleanup Verification | ✅ PASS | `205ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 6.933s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] >> [TEMPLATE] Fmt: src_v3
[T+0000] [2026-02-14T10:28:48.538-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/template/src_v3/...]
[T+0000] >> [TEMPLATE] Vet: src_v3
[T+0000] [2026-02-14T10:28:48.994-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/template/src_v3/...]
[T+0001] >> [TEMPLATE] Go Build: src_v3
[T+0001] [2026-02-14T10:28:49.550-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/template/src_v3/...]
[T+0002] >> [TEMPLATE] Lint: src_v3
[T+0002]    [LINT] Running tsc...
[T+0003] $ tsc --noEmit
[T+0004] >> [TEMPLATE] Format: src_v3
[T+0004] $ echo format-ok
[T+0004] format-ok
[T+0004] >> [TEMPLATE] Build: src_v3
[T+0004] >> [TEMPLATE] Install: src_v3
[T+0004]    [TEMPLATE] Running bun install...
[T+0004] bun install v1.3.9 (cf6cdbbb)
[T+0004] Saved lockfile
[T+0004] 
[T+0004] + @types/three@0.182.0
[T+0004] + typescript@5.9.3
[T+0004] + vite@5.4.21
[T+0004] + @xterm/addon-fit@0.11.0
[T+0004] + @xterm/xterm@6.0.0
[T+0004] + three@0.182.0
[T+0004] 
[T+0004] 23 packages installed [127.00ms]
[T+0004]    [BUILD] Running UI build...
[T+0004] $ vite build
[T+0005] vite v5.4.21 building for production...
[T+0005] transforming...
[T+0006] ✓ 20 modules transformed.
[T+0006] rendering chunks...
[T+0006] computing gzip size...
[T+0006] dist/index.html                   5.75 kB │ gzip:   1.46 kB
[T+0006] dist/assets/index-A0MCkHYl.css    4.63 kB │ gzip:   1.41 kB
[T+0006] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0006] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0006] dist/assets/index-CG3KstkX.js     0.24 kB │ gzip:   0.20 kB
[T+0006] dist/assets/index-CslqoKFv.js     0.81 kB │ gzip:   0.37 kB
[T+0006] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0006] dist/assets/index-So1TKZV8.js     9.12 kB │ gzip:   2.99 kB
[T+0006] dist/assets/index-BFRwSQ3x.js   334.99 kB │ gzip:  85.16 kB
[T+0006] dist/assets/index-CpVxsEhP.js   492.10 kB │ gzip: 124.95 kB
[T+0006] ✓ built in 941ms
[T+0006] >> [TEMPLATE] Build successful
```

### 02 Go Run

```text
result: PASS
duration: 627ms
```

#### Runner Output

```text
[T+0006] [TEST] RUN   02 Go Run
[T+0007] >> [TEMPLATE] Serve: src_v3
[T+0007] [2026-02-14T10:28:55.482-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/template/src_v3/cmd/main.go]
[T+0007] Template Server starting on http://localhost:8080
```

### 03 UI Run

```text
result: PASS
duration: 756ms
```

#### Runner Output

```text
[T+0007] [TEST] RUN   03 UI Run
[T+0007] >> [TEMPLATE] UI Run: src_v3
[T+0007] $ vite --host "127.0.0.1" --port "51003"
[T+0008] 
[T+0008]   VITE v5.4.21  ready in 109 ms
[T+0008] 
[T+0008]   ➜  Local:   http://127.0.0.1:51003/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 1.437s
```

#### Runner Output

```text
[T+0008] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0008] [2026-02-14T10:28:56.482-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-51008 --new-window --dialtone-origin=true --dialtone-role=test --headless=new]
[T+0009] [BROWSER] [log] [SectionManager] INITIAL LOAD #hero
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATING TO #hero
[T+0009] [BROWSER] [log] [SectionManager] LOADING #hero
[T+0009] [BROWSER] [log] [SectionManager] LOADED #hero
[T+0009] [BROWSER] [log] [SectionManager] START #hero
[T+0009] [BROWSER] [log] [SectionManager] NAVIGATE TO #hero
[T+0009] [BROWSER] [log] [SectionManager] RESUME #hero
[T+0009] [BROWSER] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Logs

```text
[T+0009] [log] [SectionManager] INITIAL LOAD #hero
[T+0009] [log] [SectionManager] NAVIGATING TO #hero
[T+0009] [log] [SectionManager] LOADING #hero
[T+0009] [log] [SectionManager] LOADED #hero
[T+0009] [log] [SectionManager] START #hero
[T+0009] [log] [SectionManager] NAVIGATE TO #hero
[T+0009] [log] [SectionManager] RESUME #hero
[T+0009] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0009] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 784ms
```

#### Runner Output

```text
[T+0009] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0009] >> [TEMPLATE] UI Run: src_v3
[T+0010] $ vite --host "127.0.0.1" --port "51070"
[T+0010] 
[T+0010]   VITE v5.4.21  ready in 167 ms
[T+0010] 
[T+0010]   ➜  Local:   http://127.0.0.1:51070/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 88ms
section: hero
```

#### Runner Output

```text
[T+0010] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 355ms
section: docs
```

#### Runner Output

```text
[T+0010] [TEST] RUN   07 Docs Section Validation
```

#### Browser Logs

```text
[T+0010] [log] [SectionManager] NAVIGATING TO #docs
[T+0010] [log] [SectionManager] LOADING #docs
[T+0010] [log] [SectionManager] LOADED #docs
[T+0010] [log] [SectionManager] START #docs
[T+0010] [log] [SectionManager] NAVIGATE TO #docs
[T+0010] [log] [SectionManager] RESUME #docs
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 447ms
section: table
```

#### Runner Output

```text
[T+0010] [TEST] RUN   08 Table Section Validation
```

#### Browser Logs

```text
[T+0010] [log] [SectionManager] NAVIGATING TO #table
[T+0010] [log] [SectionManager] LOADING #table
[T+0010] [log] [SectionManager] LOADED #table
[T+0010] [log] [SectionManager] START #table
[T+0010] [log] [SectionManager] NAVIGATE TO #table
[T+0010] [log] [SectionManager] RESUME #table
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 429ms
section: three
```

#### Runner Output

```text
[T+0011] [TEST] RUN   09 Three Section Validation
[T+0011] [TEST] Three hit-test hovered cube_left at (411,400)
[T+0011] [TEST] Three screenshot pixel check passed at (411,400)
```

#### Browser Logs

```text
[T+0011] [log] [SectionManager] NAVIGATING TO #three
[T+0011] [log] [SectionManager] LOADING #three
[T+0011] [log] [SectionManager] LOADED #three
[T+0011] [log] [SectionManager] START #three
[T+0011] [log] [SectionManager] NAVIGATE TO #three
[T+0011] [log] [SectionManager] RESUME #three
[T+0011] [log] [Three #three] hover cube: cube_left
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 389ms
section: xterm
```

#### Runner Output

```text
[T+0011] [TEST] RUN   10 Xterm Section Validation
```

#### Browser Logs

```text
[T+0011] [log] [SectionManager] NAVIGATING TO #xterm
[T+0011] [log] [SectionManager] LOADING #xterm
[T+0011] [log] [SectionManager] LOADED #xterm
[T+0011] [log] [SectionManager] START #xterm
[T+0011] [log] [SectionManager] NAVIGATE TO #xterm
[T+0011] [log] [SectionManager] RESUME #xterm
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 492ms
section: video
```

#### Runner Output

```text
[T+0012] [TEST] RUN   11 Video Section Validation
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #video
[T+0012] [log] [SectionManager] LOADING #video
[T+0012] [log] [SectionManager] LOADED #video
[T+0012] [log] [SectionManager] START #video
[T+0012] [log] [SectionManager] NAVIGATE TO #video
[T+0012] [log] [SectionManager] RESUME #video
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.905s
```

#### Runner Output

```text
[T+0012] [TEST] RUN   12 Lifecycle / Invariants
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #hero
[T+0012] [log] [SectionManager] NAVIGATE AWAY #video
[T+0012] [log] [SectionManager] PAUSE #video
[T+0012] [log] [SectionManager] NAVIGATE TO #hero
[T+0012] [log] [SectionManager] RESUME #hero
[T+0013] [log] [SectionManager] NAVIGATING TO #docs
[T+0013] [log] [SectionManager] NAVIGATE AWAY #hero
[T+0013] [log] [SectionManager] PAUSE #hero
[T+0013] [log] [SectionManager] NAVIGATE TO #docs
[T+0013] [log] [SectionManager] RESUME #docs
[T+0013] [log] [SectionManager] NAVIGATING TO #table
[T+0013] [log] [SectionManager] NAVIGATE AWAY #docs
[T+0013] [log] [SectionManager] PAUSE #docs
[T+0013] [log] [SectionManager] NAVIGATE TO #table
[T+0013] [log] [SectionManager] RESUME #table
[T+0013] [log] [SectionManager] NAVIGATING TO #three
[T+0013] [log] [SectionManager] NAVIGATE AWAY #table
[T+0013] [log] [SectionManager] PAUSE #table
[T+0013] [log] [SectionManager] NAVIGATE TO #three
[T+0013] [log] [SectionManager] RESUME #three
[T+0014] [log] [SectionManager] NAVIGATING TO #xterm
[T+0014] [log] [SectionManager] NAVIGATE AWAY #three
[T+0014] [log] [SectionManager] PAUSE #three
[T+0014] [log] [SectionManager] NAVIGATE TO #xterm
[T+0014] [log] [SectionManager] RESUME #xterm
[T+0014] [log] [Three #three] hover cube: none
[T+0014] [log] [SectionManager] NAVIGATING TO #video
[T+0014] [log] [SectionManager] NAVIGATE AWAY #xterm
[T+0014] [log] [SectionManager] PAUSE #xterm
[T+0014] [log] [SectionManager] NAVIGATE TO #video
[T+0014] [log] [SectionManager] RESUME #video
```

### 13 Cleanup Verification

```text
result: PASS
duration: 205ms
```

#### Runner Output

```text
[T+0014] [TEST] RUN   13 Cleanup Verification
[T+0014] Cleaning up stale process on port 8080 (PID: 35080)...
```

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1.png`
- `screenshots/test_step_2.png`
- `screenshots/test_step_3.png`
- `screenshots/test_step_4.png`
- `screenshots/test_step_5.png`
- `screenshots/test_step_6.png`
