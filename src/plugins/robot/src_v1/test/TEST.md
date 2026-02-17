# Robot Plugin src_v1 Test Report

**Generated at:** Tue, 17 Feb 2026 11:10:00 -0800
**Version:** `src_v1`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `18.568s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `6.813s` |
| 02 Go Run (Mock Server Check) | ✅ PASS | `607ms` |
| 03 UI Run | ✅ PASS | `776ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `4.68s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `779ms` |
| 06 Hero Section Validation | ✅ PASS | `156ms` |
| 07 Docs Section Validation | ✅ PASS | `347ms` |
| 08 Table Section Validation | ✅ PASS | `392ms` |
| 09 Three Section Validation | ✅ PASS | `371ms` |
| 10 Xterm Section Validation | ✅ PASS | `376ms` |
| 11 Video Section Validation | ✅ PASS | `572ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.535s` |
| 13 Menu Navigation Validation | ✅ PASS | `535ms` |
| 14 Cleanup Verification | ✅ PASS | `257ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 6.813s
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
[T+0000] [2026-02-17T11:09:42.676-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/robot/src_v1/...]
[T+0001] [2026-02-17T11:09:43.200-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/robot/src_v1/...]
[T+0002] [2026-02-17T11:09:43.809-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/robot/src_v1/...]
[T+0002] $ tsc --noEmit
[T+0004] $ echo format-ok
[T+0004] format-ok
[T+0004] >> [Robot] Building UI: src_v1
[T+0004] $ vite build
[T+0004] vite v5.4.21 building for production...
[T+0004] transforming...
[T+0005] ✓ 21 modules transformed.
[T+0005] rendering chunks...
[T+0006] computing gzip size...
[T+0006] dist/index.html                   4.08 kB │ gzip:   1.20 kB
[T+0006] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0006] dist/assets/index-BjZEH3HJ.css   11.16 kB │ gzip:   2.92 kB
[T+0006] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0006] dist/assets/index-DP7yMgmI.js     0.79 kB │ gzip:   0.44 kB
[T+0006] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0006] dist/assets/index-Cv3Dw90O.js     3.06 kB │ gzip:   1.12 kB
[T+0006] dist/assets/index-CWPYaS-O.js   192.55 kB │ gzip:  60.22 kB
[T+0006] dist/assets/index-CP9Roacu.js   334.99 kB │ gzip:  85.16 kB
[T+0006] dist/assets/index-D5ohG2CK.js   498.24 kB │ gzip: 126.62 kB
[T+0006] ✓ built in 1.35s
[T+0006] >> [Robot] Building Dialtone Binary into src/plugins/robot/bin
[T+0006] [2026-02-17T11:09:48.057-08:00 | INFO | build.go:RunBuild:110] Building Dialtone for Linux amd64 using Podman (gcc, g++)...
[T+0006] [2026-02-17T11:09:48.106-08:00 | INFO | build.go:RunBuild:110] Using optimized 'dialtone-builder' image (skipping apt-get install)
[T+0006] [2026-02-17T11:09:48.106-08:00 | INFO | build.go:RunBuild:110] Running: podman [run --rm -v /home/user/dialtone:/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 -e CC=gcc -e CXX=g++ dialtone-builder bash -c go build -buildvcs=false -ldflags="-s -w" -trimpath -tags no_duckdb -o src/plugins/robot/bin/dialtone-amd64 src/cmd/dialtone/main.go]
[T+0006] [2026-02-17T11:09:48.549-08:00 | INFO | build.go:RunBuild:110] Build successful: bin/dialtone-amd64
```

### 02 Go Run (Mock Server Check)

```text
result: PASS
duration: 607ms
```

#### Runner Output

```text
[T+0006] [TEST] RUN   02 Go Run (Mock Server Check)
[T+0007] [2026-02-17T11:09:48.924-08:00 | INFO | robot.go:RunRobot:41] [WARNING] Process is not running as a systemd service. Consider running via systemctl.
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:RunStart:67] NATS server started on port 4222 (local only)
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:runLocalOnly:89] NATS WS proxy ports: external=4223 internal=4223
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | asm_amd64.s:goexit:1693] Starting Mock Mavlink Publisher...
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:runLocalOnly:89] Using provided static web assets
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:runLocalOnly:91] NATS WS proxy ports: external=4223 internal=4223
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:runLocalOnly:91] Using provided static web assets
[T+0007] [2026-02-17T11:09:49.090-08:00 | INFO | start.go:RunStart:67] Web UI (Local Only): Serving at http://0.0.0.0:8080
```

### 03 UI Run

```text
result: PASS
duration: 776ms
```

#### Runner Output

```text
[T+0007] [TEST] RUN   03 UI Run
[T+0007] $ vite --host "127.0.0.1" --port "43638"
[T+0008] 
[T+0008]   VITE v5.4.21  ready in 113 ms
[T+0008] 
[T+0008]   ➜  Local:   http://127.0.0.1:43638/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 4.68s
```

#### Runner Output

```text
[T+0008] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0008] [2026-02-17T11:09:49.969-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /mnt/c/Program Files/Google/Chrome/Application/chrome.exe [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-port-44670 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080]
```

#### Browser Logs

```text
[T+0012] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0012] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 779ms
```

#### Runner Output

```text
[T+0012] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0013] $ vite --host "127.0.0.1" --port "44326"
[T+0013] 
[T+0013]   VITE v5.4.21  ready in 128 ms
[T+0013] 
[T+0013]   ➜  Local:   http://127.0.0.1:44326/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 156ms
section: hero
```

#### Runner Output

```text
[T+0013] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 347ms
section: docs
```

#### Runner Output

```text
[T+0013] [TEST] RUN   07 Docs Section Validation
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 392ms
section: table
```

#### Runner Output

```text
[T+0014] [TEST] RUN   08 Table Section Validation
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 371ms
section: three
```

#### Runner Output

```text
[T+0014] [TEST] RUN   09 Three Section Validation
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 376ms
section: xterm
```

#### Runner Output

```text
[T+0014] [TEST] RUN   10 Xterm Section Validation
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 572ms
section: video
```

#### Runner Output

```text
[T+0015] [TEST] RUN   11 Video Section Validation
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.535s
```

#### Runner Output

```text
[T+0015] [TEST] RUN   12 Lifecycle / Invariants
```

### 13 Menu Navigation Validation

```text
result: PASS
duration: 535ms
```

#### Runner Output

```text
[T+0017] [TEST] RUN   13 Menu Navigation Validation
```

![13 Menu Navigation Validation sequence](../screenshots/menu_nav_grid.png)

### 14 Cleanup Verification

```text
result: PASS
duration: 257ms
```

#### Runner Output

```text
[T+0018] [TEST] RUN   14 Cleanup Verification
[T+0018] Cleaning up Dialtone-related process on port 8080 (PID: 353909, Name: main)...
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
