# Template Plugin v3 Test Report

**Generated at:** Fri, 13 Feb 2026 10:34:28 -0800
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ❌ FAIL
**Total Time:** `12.836s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `7.327s` |
| 02 Go Run | ✅ PASS | `1.039s` |
| 03 UI Run | ✅ PASS | `758ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `1.645s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `772ms` |
| 06 Hero Section Validation | ✅ PASS | `72ms` |
| 07 Docs Section Validation | ✅ PASS | `305ms` |
| 08 Table Section Validation | ✅ PASS | `339ms` |
| 09 Three Section Validation | ✅ PASS | `305ms` |
| 10 Xterm Section Validation | ❌ FAIL | `273ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 7.327s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] >> [TEMPLATE] Fmt: src_v3
[T+0000] [2026-02-13T10:34:15.946-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/template/src_v3/...]
[T+0000] >> [TEMPLATE] Vet: src_v3
[T+0001] [2026-02-13T10:34:16.548-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/template/src_v3/...]
[T+0001] >> [TEMPLATE] Go Build: src_v3
[T+0001] [2026-02-13T10:34:17.230-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/template/src_v3/...]
[T+0003] >> [TEMPLATE] Lint: src_v3
[T+0003] $ tsc --noEmit
[T+0005] >> [TEMPLATE] Format: src_v3
[T+0005] $ echo format-ok
[T+0005] format-ok
[T+0005] >> [TEMPLATE] Build: src_v3
[T+0005] $ vite build
[T+0006] vite v5.4.21 building for production...
[T+0006] transforming...
[T+0007] ✓ 22 modules transformed.
[T+0007] rendering chunks...
[T+0007] computing gzip size...
[T+0007] dist/index.html                   5.75 kB │ gzip:   1.46 kB
[T+0007] dist/assets/index-A0MCkHYl.css    4.63 kB │ gzip:   1.41 kB
[T+0007] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0007] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0007] dist/assets/index-CG3KstkX.js     0.24 kB │ gzip:   0.20 kB
[T+0007] dist/assets/index-CslqoKFv.js     0.81 kB │ gzip:   0.37 kB
[T+0007] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0007] dist/assets/index-nleHRQBf.js     9.12 kB │ gzip:   2.99 kB
[T+0007] dist/assets/index-BFRwSQ3x.js   334.99 kB │ gzip:  85.16 kB
[T+0007] dist/assets/index-BFuguuOd.js   488.28 kB │ gzip: 124.04 kB
[T+0007] ✓ built in 693ms
```

### 02 Go Run

```text
result: PASS
duration: 1.039s
```

#### Runner Output

```text
[T+0007] [TEST] RUN   02 Go Run
[T+0007] >> [TEMPLATE] Serve: src_v3
[T+0007] [2026-02-13T10:34:23.312-08:00 | INFO | go.go:RunGo:33] Running: go [run src/plugins/template/src_v3/cmd/main.go]
[T+0008] Template Server starting on http://localhost:8080
```

### 03 UI Run

```text
result: PASS
duration: 758ms
```

#### Runner Output

```text
[T+0008] [TEST] RUN   03 UI Run
[T+0008] >> [TEMPLATE] UI Run: src_v3
[T+0008] $ vite --host "127.0.0.1" --port "63331"
[T+0008] 
[T+0008]   VITE v5.4.21  ready in 85 ms
[T+0008] 
[T+0008]   ➜  Local:   http://127.0.0.1:63331/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 1.645s
```

#### Runner Output

```text
[T+0009] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0009] [2026-02-13T10:34:24.553-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/tim/code/dialtone/.chrome_data/dialtone-chrome-test-port-63336 --new-window --dialtone-origin=true --dialtone-role=test --headless=new]
[T+0010] [BROWSER] [log] [SectionManager] INITIAL LOAD #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [BROWSER] [log] [SectionManager] LOADING #hero
[T+0010] [BROWSER] [log] [SectionManager] LOADED #hero
[T+0010] [BROWSER] [log] [SectionManager] START #hero
[T+0010] [BROWSER] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [BROWSER] [log] [SectionManager] RESUME #hero
[T+0010] [BROWSER] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Logs

```text
[T+0010] [log] [SectionManager] INITIAL LOAD #hero
[T+0010] [log] [SectionManager] NAVIGATING TO #hero
[T+0010] [log] [SectionManager] LOADING #hero
[T+0010] [log] [SectionManager] LOADED #hero
[T+0010] [log] [SectionManager] START #hero
[T+0010] [log] [SectionManager] NAVIGATE TO #hero
[T+0010] [log] [SectionManager] RESUME #hero
[T+0010] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0010] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 772ms
```

#### Runner Output

```text
[T+0010] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0011] >> [TEMPLATE] UI Run: src_v3
[T+0011] $ vite --host "127.0.0.1" --port "63361"
[T+0011] 
[T+0011]   VITE v5.4.21  ready in 87 ms
[T+0011] 
[T+0011]   ➜  Local:   http://127.0.0.1:63361/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 72ms
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
duration: 305ms
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
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 339ms
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
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 305ms
section: three
```

#### Runner Output

```text
[T+0012] [TEST] RUN   09 Three Section Validation
```

#### Browser Logs

```text
[T+0012] [log] [SectionManager] NAVIGATING TO #three
[T+0012] [log] [SectionManager] LOADING #three
[T+0012] [log] [SectionManager] LOADED #three
[T+0012] [log] [SectionManager] START #three
[T+0012] [log] [SectionManager] NAVIGATE TO #three
[T+0012] [log] [SectionManager] RESUME #three
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: FAIL
duration: 273ms
section: xterm
error: aria-label Xterm Terminal is outside viewport (rect top=0.0 left=0.0 bottom=744.0 right=1297.0 viewport=1280.0x800.0)
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
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

## Artifacts

- `test.log`
- `error.log`
- `screenshots/test_step_1.png`
- `screenshots/test_step_2.png`
- `screenshots/test_step_3.png`
- `screenshots/test_step_4.png`
- `screenshots/test_step_5.png`
