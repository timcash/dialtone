# Template Plugin v3 Test Report

**Generated at:** Sat, 14 Feb 2026 11:25:07 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `14.787s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `7.421s` |
| 02 Go Run | ✅ PASS | `630ms` |
| 03 UI Run | ✅ PASS | `756ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `1.533s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `781ms` |
| 06 Hero Section Validation | ✅ PASS | `76ms` |
| 07 Docs Section Validation | ✅ PASS | `297ms` |
| 08 Table Section Validation | ✅ PASS | `304ms` |
| 09 Three Section Validation | ✅ PASS | `442ms` |
| 10 Xterm Section Validation | ✅ PASS | `463ms` |
| 11 Video Section Validation | ✅ PASS | `356ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.523s` |
| 13 Cleanup Verification | ✅ PASS | `204ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 7.421s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] >> [TEMPLATE] Fmt: src_v3
[T+0000] [2026-02-14T11:24:53.175-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/template/src_v3/...]
[T+0000] >> [TEMPLATE] Vet: src_v3
[T+0000] [2026-02-14T11:24:53.616-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/template/src_v3/...]
[T+0001] >> [TEMPLATE] Go Build: src_v3
[T+0001] [2026-02-14T11:24:54.180-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/template/src_v3/...]
[T+0003] >> [TEMPLATE] Lint: src_v3
[T+0003]    [LINT] Running tsc...
[T+0003] $ tsc --noEmit
[T+0004] >> [TEMPLATE] Format: src_v3
[T+0004] $ echo format-ok
[T+0004] format-ok
[T+0004] >> [TEMPLATE] Build: src_v3
[T+0004] >> [TEMPLATE] Install: src_v3
[T+0004]    [TEMPLATE] Running bun install...
[T+0005] bun install v1.3.9 (cf6cdbbb)
[T+0005] 
[T+0005] + @types/three@0.182.0
[T+0005] + typescript@5.9.3
[T+0005] + vite@5.4.21
[T+0005] + @xterm/addon-fit@0.11.0
[T+0005] + @xterm/xterm@6.0.0
[T+0005] + three@0.182.0
[T+0005] 
[T+0005] 23 packages installed [147.00ms]
[T+0005] Saved lockfile
[T+0005]    [BUILD] Running UI build...
[T+0005] $ vite build
[T+0006] vite v5.4.21 building for production...
[T+0006] transforming...
[T+0007] ✓ 20 modules transformed.
[T+0007] rendering chunks...
[T+0007] computing gzip size...
[T+0007] dist/index.html                   2.80 kB │ gzip:   0.95 kB
[T+0007] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0007] dist/assets/index-CbitW_B7.css    5.77 kB │ gzip:   1.70 kB
[T+0007] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0007] dist/assets/index-CslqoKFv.js     0.81 kB │ gzip:   0.37 kB
[T+0007] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0007] dist/assets/index-CW0cA2Fg.js     1.96 kB │ gzip:   1.04 kB
[T+0007] dist/assets/index-_v9y-Lws.js     9.61 kB │ gzip:   3.15 kB
[T+0007] dist/assets/index-BFRwSQ3x.js   334.99 kB │ gzip:  85.16 kB
[T+0007] dist/assets/index-De5tMUio.js   492.59 kB │ gzip: 125.04 kB
[T+0007] ✓ built in 943ms
[T+0007] >> [TEMPLATE] Build successful
```

### 02 Go Run

```text
result: PASS
duration: 630ms
```

#### Runner Output

```text
[T+0007] [TEST] RUN   02 Go Run
[T+0007] >> [TEMPLATE] Serve: src_v3
[T+0007] [2026-02-14T11:25:00.620-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/template/src_v3/cmd/main.go]
[T+0007] Template Server starting on http://localhost:8080
```

### 03 UI Run

```text
result: PASS
duration: 756ms
```

#### Runner Output

```text
[T+0008] [TEST] RUN   03 UI Run
[T+0008] >> [TEMPLATE] UI Run: src_v3
[T+0008] $ vite --host "127.0.0.1" --port "56317"
[T+0008] 
[T+0008]   VITE v5.4.21  ready in 110 ms
[T+0008] 
[T+0008]   ➜  Local:   http://127.0.0.1:56317/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 1.533s
```

#### Runner Output

```text
[T+0008] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0008] [2026-02-14T11:25:01.616-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/dev/code/dialtone/.chrome_data/dialtone-chrome-test-port-56326 --new-window --dialtone-origin=true --dialtone-role=test --headless=new]
[T+0010] [BROWSER] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/ hash=(none) active=(none) target=hero
[T+0010] [BROWSER] [log] [SectionManager] URL SYNC #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [BROWSER] [log] [SectionManager] LOADING #hero
[T+0010] [BROWSER] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/ hash=(none) active=(none) target=hero
[T+0010] [BROWSER] [log] [SectionManager] URL SYNC #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [BROWSER] [log] [SectionManager] LOADED #hero
[T+0010] [BROWSER] [log] [SectionManager] START #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [BROWSER] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Logs

```text
[T+0010] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/ hash=(none) active=(none) target=hero
[T+0010] [log] [SectionManager] URL SYNC #hero
[T+0010] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [log] [SectionManager] LOADING #hero
[T+0010] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/ hash=(none) active=(none) target=hero
[T+0010] [log] [SectionManager] URL SYNC #hero
[T+0010] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [log] [SectionManager] LOADED #hero
[T+0010] [log] [SectionManager] START #hero
[T+0010] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0010] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 781ms
```

#### Runner Output

```text
[T+0010] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0010] >> [TEMPLATE] UI Run: src_v3
[T+0010] $ vite --host "127.0.0.1" --port "56391"
[T+0010] 
[T+0010]   VITE v5.4.21  ready in 114 ms
[T+0010] 
[T+0010]   ➜  Local:   http://127.0.0.1:56391/
```

#### Browser Logs

```text
[T+0010] [log] [SectionManager] RESUME #hero
[T+0010] [log] [SectionManager] RESUME #hero
[T+0010] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#hero hash=hero active=hero target=hero
```

### 06 Hero Section Validation

```text
result: PASS
duration: 76ms
section: hero
```

#### Runner Output

```text
[T+0011] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 297ms
section: docs
```

#### Runner Output

```text
[T+0011] [TEST] RUN   07 Docs Section Validation
```

#### Browser Logs

```text
[T+0011] [log] [SectionManager] NAVIGATING TO #docs
[T+0011] [log] [SectionManager] LOADING #docs
[T+0011] [log] [SectionManager] LOADED #docs
[T+0011] [log] [SectionManager] START #docs
[T+0011] [log] [SectionManager] NAVIGATE TO #docs
[T+0011] [log] [SectionManager] RESUME #docs
[T+0011] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#docs hash=docs active=docs target=docs
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 304ms
section: table
```

#### Runner Output

```text
[T+0011] [TEST] RUN   08 Table Section Validation
```

#### Browser Logs

```text
[T+0011] [log] [SectionManager] NAVIGATING TO #table
[T+0011] [log] [SectionManager] LOADING #table
[T+0011] [log] [SectionManager] LOADED #table
[T+0011] [log] [SectionManager] START #table
[T+0011] [log] [SectionManager] NAVIGATE TO #table
[T+0011] [log] [SectionManager] RESUME #table
[T+0011] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#table hash=table active=table target=table
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 442ms
section: three
```

#### Runner Output

```text
[T+0011] [TEST] RUN   09 Three Section Validation
```

#### Browser Logs

```text
[T+0011] [log] [SectionManager] NAVIGATING TO #three
[T+0011] [log] [SectionManager] LOADING #three
[T+0011] [log] [SectionManager] LOADED #three
[T+0011] [log] [SectionManager] START #three
[T+0011] [log] [SectionManager] NAVIGATE TO #three
[T+0011] [log] [SectionManager] RESUME #three
[T+0011] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#three hash=three active=three target=three
[T+0012] [log] [Three #three] touch cube: cube_left
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 463ms
section: xterm
```

#### Runner Output

```text
[T+0012] [TEST] RUN   10 Xterm Section Validation
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #xterm
[T+0012] [log] [SectionManager] LOADING #xterm
[T+0012] [log] [SectionManager] LOADED #xterm
[T+0012] [log] [SectionManager] START #xterm
[T+0012] [log] [SectionManager] NAVIGATE TO #xterm
[T+0012] [log] [SectionManager] RESUME #xterm
[T+0012] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#xterm hash=xterm active=xterm target=xterm
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 356ms
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
[T+0012] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#video hash=video active=video target=video
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.523s
```

#### Runner Output

```text
[T+0013] [TEST] RUN   12 Lifecycle / Invariants
```

#### Browser Logs

```text
[T+0013] [log] [SectionManager] NAVIGATING TO #hero
[T+0013] [log] [SectionManager] NAVIGATE AWAY #video
[T+0013] [log] [SectionManager] PAUSE #video
[T+0013] [log] [SectionManager] NAVIGATE TO #hero
[T+0013] [log] [SectionManager] RESUME #hero
[T+0013] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#hero hash=hero active=hero target=hero
[T+0013] [log] [SectionManager] NAVIGATING TO #docs
[T+0013] [log] [SectionManager] NAVIGATE AWAY #hero
[T+0013] [log] [SectionManager] PAUSE #hero
[T+0013] [log] [SectionManager] NAVIGATE TO #docs
[T+0013] [log] [SectionManager] RESUME #docs
[T+0013] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#docs hash=docs active=docs target=docs
[T+0013] [log] [SectionManager] NAVIGATING TO #table
[T+0013] [log] [SectionManager] NAVIGATE AWAY #docs
[T+0013] [log] [SectionManager] PAUSE #docs
[T+0013] [log] [SectionManager] NAVIGATE TO #table
[T+0013] [log] [SectionManager] RESUME #table
[T+0013] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#table hash=table active=table target=table
[T+0013] [log] [SectionManager] NAVIGATING TO #three
[T+0013] [log] [SectionManager] NAVIGATE AWAY #table
[T+0013] [log] [SectionManager] PAUSE #table
[T+0013] [log] [SectionManager] NAVIGATE TO #three
[T+0013] [log] [SectionManager] RESUME #three
[T+0013] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#three hash=three active=three target=three
[T+0014] [log] [SectionManager] NAVIGATING TO #xterm
[T+0014] [log] [SectionManager] NAVIGATE AWAY #three
[T+0014] [log] [SectionManager] PAUSE #three
[T+0014] [log] [SectionManager] NAVIGATE TO #xterm
[T+0014] [log] [SectionManager] RESUME #xterm
[T+0014] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#xterm hash=xterm active=xterm target=xterm
[T+0014] [log] [SectionManager] NAVIGATING TO #video
[T+0014] [log] [SectionManager] NAVIGATE AWAY #xterm
[T+0014] [log] [SectionManager] PAUSE #xterm
[T+0014] [log] [SectionManager] NAVIGATE TO #video
[T+0014] [log] [SectionManager] RESUME #video
[T+0014] [log] [SectionManager] URL PAGE http://127.0.0.1:8080/#video hash=video active=video target=video
```

### 13 Cleanup Verification

```text
result: PASS
duration: 204ms
```

#### Runner Output

```text
[T+0014] [TEST] RUN   13 Cleanup Verification
[T+0014] Cleaning up stale process on port 8080 (PID: 53814)...
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
