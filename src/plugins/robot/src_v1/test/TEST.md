# Robot Plugin src_v1 Test Report

**Generated at:** Tue, 17 Feb 2026 15:11:03 -0800
**Version:** `src_v1`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `21.323s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `9.598s` |
| 02 Go Run (Mock Server Check) | ✅ PASS | `638ms` |
| 03 UI Run | ✅ PASS | `775ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `4.751s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `773ms` |
| 06 Hero Section Validation | ✅ PASS | `163ms` |
| 07 Docs Section Validation | ✅ PASS | `346ms` |
| 08 Table Section Validation | ✅ PASS | `362ms` |
| 09 Three Section Validation | ✅ PASS | `365ms` |
| 10 Xterm Section Validation | ✅ PASS | `385ms` |
| 11 Video Section Validation | ✅ PASS | `535ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.536s` |
| 13 Menu Navigation Validation | ✅ PASS | `498ms` |
| 14 Cleanup Verification | ✅ PASS | `242ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 9.598s
```

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] bun install v1.3.9 (cf6cdbbb)
[T+0000] 
[T+0000] + @types/three@0.182.0
[T+0000] + typescript@5.9.3
[T+0000] + vite@5.4.21
[T+0000] + @xterm/addon-fit@0.11.0
[T+0000] + @xterm/xterm@6.0.0
[T+0000] + nats.ws@1.30.3
[T+0000] + three@0.182.0
[T+0000] 
[T+0000] 26 packages installed [42.00ms]
[T+0000] Saved lockfile
[T+0000] [2026-02-17T15:10:42.995-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/robot/src_v1/...]
[T+0001] [2026-02-17T15:10:43.499-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/robot/src_v1/...]
[T+0002] [2026-02-17T15:10:44.106-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/robot/src_v1/...]
[T+0002] $ tsc --noEmit
[T+0003] $ echo format-ok
[T+0003] format-ok
[T+0004] >> [Robot] Building UI: src_v1
[T+0004] $ vite build
[T+0004] vite v5.4.21 building for production...
[T+0004] transforming...
[T+0005] ✓ 22 modules transformed.
[T+0005] rendering chunks...
[T+0005] computing gzip size...
[T+0005] dist/index.html                   4.38 kB │ gzip:   1.28 kB
[T+0005] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0005] dist/assets/index-CqzJoHVm.css   11.44 kB │ gzip:   3.00 kB
[T+0005] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0005] dist/assets/index-DP7yMgmI.js     0.79 kB │ gzip:   0.44 kB
[T+0005] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0005] dist/assets/index-C49NKtLT.js     2.85 kB │ gzip:   1.01 kB
[T+0005] dist/assets/index-G-H3Df-3.js   193.42 kB │ gzip:  60.49 kB
[T+0005] dist/assets/index-CP9Roacu.js   334.99 kB │ gzip:  85.16 kB
[T+0005] dist/assets/index-O1tcAnm5.js   499.85 kB │ gzip: 127.71 kB
[T+0005] ✓ built in 1.18s
[T+0005] >> [Robot] Building Dialtone Binary into src/plugins/robot/bin
[T+0005] [2026-02-17T15:10:47.836-08:00 | INFO | build.go:RunBuild:110] Building Dialtone for Linux amd64 using Podman (gcc, g++)...
[T+0005] [2026-02-17T15:10:47.856-08:00 | INFO | build.go:RunBuild:110] Using optimized 'dialtone-builder' image (skipping apt-get install)
[T+0005] [2026-02-17T15:10:47.856-08:00 | INFO | build.go:RunBuild:110] Running: podman [run --rm -v /home/user/dialtone:/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 -e CC=gcc -e CXX=g++ dialtone-builder bash -c go build -buildvcs=false -ldflags="-s -w" -trimpath -tags no_duckdb -o src/plugins/robot/bin/dialtone-amd64 src/cmd/dialtone/main.go]
[T+0009] [2026-02-17T15:10:51.692-08:00 | INFO | build.go:RunBuild:110] Build successful: bin/dialtone-amd64
```

### 02 Go Run (Mock Server Check)

```text
result: PASS
duration: 638ms
```

#### Runner Output

```text
[T+0009] [TEST] RUN   02 Go Run (Mock Server Check)
[T+0009] [2026-02-17T15:10:52.041-08:00 | INFO | robot.go:RunRobot:41] [WARNING] Process is not running as a systemd service. Consider running via systemctl.
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:RunStart:67] NATS server started on port 4222 (local only)
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:runLocalOnly:89] NATS WS proxy ports: external=4223 internal=4223
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | asm_amd64.s:goexit:1693] Starting Mock Mavlink Publisher...
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:runLocalOnly:89] Using provided static web assets
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:runLocalOnly:91] NATS WS proxy ports: external=4223 internal=4223
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:runLocalOnly:91] Using provided static web assets
[T+0010] [2026-02-17T15:10:52.202-08:00 | INFO | start.go:RunStart:67] Web UI (Local Only): Serving at http://0.0.0.0:8080
```

### 03 UI Run

```text
result: PASS
duration: 775ms
```

#### Runner Output

```text
[T+0010] [TEST] RUN   03 UI Run
[T+0010] $ vite --host "127.0.0.1" --port "44498"
[T+0010] 
[T+0010]   VITE v5.4.21  ready in 106 ms
[T+0010] 
[T+0010]   ➜  Local:   http://127.0.0.1:44498/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 4.751s
```

#### Runner Output

```text
[T+0011] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0011] [2026-02-17T15:10:53.147-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /mnt/c/Program Files/Google/Chrome/Application/chrome.exe [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-port-44182 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080]
```

#### Browser Logs

```text
[T+0015] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0015] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 773ms
```

#### Runner Output

```text
[T+0015] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0016] $ vite --host "127.0.0.1" --port "44832"
[T+0016] 
[T+0016]   VITE v5.4.21  ready in 101 ms
[T+0016] 
[T+0016]   ➜  Local:   http://127.0.0.1:44832/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 163ms
section: hero
```

#### Runner Output

```text
[T+0016] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 346ms
section: docs
```

#### Runner Output

```text
[T+0016] [TEST] RUN   07 Docs Section Validation
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 362ms
section: table
```

#### Runner Output

```text
[T+0017] [TEST] RUN   08 Table Section Validation
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 365ms
section: three
```

#### Runner Output

```text
[T+0017] [TEST] RUN   09 Three Section Validation
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 385ms
section: xterm
```

#### Runner Output

```text
[T+0017] [TEST] RUN   10 Xterm Section Validation
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 535ms
section: video
```

#### Runner Output

```text
[T+0018] [TEST] RUN   11 Video Section Validation
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.536s
```

#### Runner Output

```text
[T+0018] [TEST] RUN   12 Lifecycle / Invariants
```

### 13 Menu Navigation Validation

```text
result: PASS
duration: 498ms
```

#### Runner Output

```text
[T+0020] [TEST] RUN   13 Menu Navigation Validation
```

![13 Menu Navigation Validation sequence](../screenshots/menu_nav_grid.png)

### 14 Cleanup Verification

```text
result: PASS
duration: 242ms
```

#### Runner Output

```text
[T+0021] [TEST] RUN   14 Cleanup Verification
[T+0021] Cleaning up Dialtone-related process on port 8080 (PID: 392618, Name: main)...
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
- `screenshots/menu_nav_grid.png`
- `screenshots/menu_1_hero.png`
- `screenshots/menu_2_open.png`
- `screenshots/menu_3_telemetry.png`
