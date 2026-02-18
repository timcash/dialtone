# Robot Plugin src_v1 Test Report

**Generated at:** Tue, 17 Feb 2026 19:29:16 -0800
**Version:** `src_v1`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `19.584s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `8.057s` |
| 02 Go Run (Mock Server Check) | ✅ PASS | `586ms` |
| 03 UI Run | ✅ PASS | `525ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `4.601s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `776ms` |
| 06 Hero Section Validation | ✅ PASS | `163ms` |
| 07 Docs Section Validation | ✅ PASS | `343ms` |
| 08 Table Section Validation | ✅ PASS | `373ms` |
| 09 Three Section Validation | ✅ PASS | `433ms` |
| 10 Xterm Section Validation | ✅ PASS | `397ms` |
| 11 Video Section Validation | ✅ PASS | `515ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.521s` |
| 13 Menu Navigation Validation | ✅ PASS | `713ms` |
| 14 Cleanup Verification | ✅ PASS | `240ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 8.057s
```

#### Step Story

Preflight checks passed.

#### Runner Output

```text
[T+0000] [TEST] RUN   01 Preflight (Go/UI)
[T+0000] bun install v1.3.9 (cf6cdbbb)
[T+0000] Saved lockfile
[T+0000] 
[T+0000] + @types/three@0.182.0
[T+0000] + typescript@5.9.3
[T+0000] + vite@5.4.21
[T+0000] + @xterm/addon-fit@0.11.0
[T+0000] + @xterm/xterm@6.0.0
[T+0000] + nats.ws@1.30.3
[T+0000] + three@0.182.0
[T+0000] 
[T+0000] 26 packages installed [109.00ms]
[T+0000] [2026-02-17T19:28:57.358-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/robot/src_v1/...]
[T+0000] src/plugins/robot/src_v1/test/test_ctx.go
[T+0001] [2026-02-17T19:28:57.821-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/robot/src_v1/...]
[T+0001] [2026-02-17T19:28:58.462-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/robot/src_v1/...]
[T+0004] $ tsc --noEmit
[T+0005] $ echo format-ok
[T+0005] format-ok
[T+0005] >> [Robot] Building UI: src_v1
[T+0005] $ vite build
[T+0005] vite v5.4.21 building for production...
[T+0006] transforming...
[T+0006] ✓ 22 modules transformed.
[T+0007] rendering chunks...
[T+0007] computing gzip size...
[T+0007] dist/index.html                   4.88 kB │ gzip:   1.50 kB
[T+0007] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0007] dist/assets/index-DC1O7ztS.css   11.48 kB │ gzip:   3.00 kB
[T+0007] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0007] dist/assets/index-DP7yMgmI.js     0.79 kB │ gzip:   0.44 kB
[T+0007] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0007] dist/assets/index-ClDzHpV3.js     2.85 kB │ gzip:   1.01 kB
[T+0007] dist/assets/index-BTVTkvor.js   193.75 kB │ gzip:  60.51 kB
[T+0007] dist/assets/index-CP9Roacu.js   334.99 kB │ gzip:  85.16 kB
[T+0007] dist/assets/index-CuXPKxkZ.js   499.85 kB │ gzip: 127.71 kB
[T+0007] ✓ built in 1.12s
[T+0007] >> [Robot] Building Dialtone Binary into src/plugins/robot/bin
[T+0007] [2026-02-17T19:29:03.902-08:00 | INFO | build.go:RunBuild:110] Building Dialtone for Linux amd64 using Podman (gcc, g++)...
[T+0007] [2026-02-17T19:29:03.920-08:00 | INFO | build.go:RunBuild:110] Using optimized 'dialtone-builder' image (skipping apt-get install)
[T+0007] [2026-02-17T19:29:03.920-08:00 | INFO | build.go:RunBuild:110] Running: podman [run --rm -v /home/user/dialtone:/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 -e CC=gcc -e CXX=g++ dialtone-builder bash -c go build -buildvcs=false -ldflags="-s -w" -trimpath -tags no_duckdb -o src/plugins/robot/bin/dialtone-amd64 src/cmd/dialtone/main.go]
[T+0008] [2026-02-17T19:29:04.542-08:00 | INFO | build.go:RunBuild:110] Build successful: bin/dialtone-amd64
```

### 02 Go Run (Mock Server Check)

```text
result: PASS
duration: 586ms
```

#### Step Story

Go server running.

#### Runner Output

```text
[T+0008] [TEST] RUN   02 Go Run (Mock Server Check)
[T+0008] [2026-02-17T19:29:04.812-08:00 | INFO | robot.go:RunRobot:41] [WARNING] Process is not running as a systemd service. Consider running via systemctl.
[T+0008] [2026-02-17T19:29:04.972-08:00 | INFO | start.go:RunStart:67] NATS server started on port 4222 (local only)
[T+0008] [2026-02-17T19:29:04.972-08:00 | INFO | asm_amd64.s:goexit:1693] Starting Mock Mavlink Publisher...
[T+0008] [2026-02-17T19:29:04.972-08:00 | INFO | start.go:runLocalOnly:89] NATS WS proxy ports: external=4223 internal=4223
[T+0008] [2026-02-17T19:29:04.972-08:00 | INFO | start.go:runLocalOnly:89] Using provided static web assets
[T+0008] [2026-02-17T19:29:04.972-08:00 | INFO | start.go:runLocalOnly:91] NATS WS proxy ports: external=4223 internal=4223
[T+0008] [2026-02-17T19:29:04.973-08:00 | INFO | start.go:runLocalOnly:91] Using provided static web assets
[T+0008] [2026-02-17T19:29:04.973-08:00 | INFO | start.go:RunStart:67] Web UI (Local Only): Serving at http://0.0.0.0:8080
```

### 03 UI Run

```text
result: PASS
duration: 525ms
```

#### Step Story

UI server check passed.

#### Runner Output

```text
[T+0008] [TEST] RUN   03 UI Run
[T+0009] $ vite --host "127.0.0.1" --port "44424"
[T+0009] 
[T+0009]   VITE v5.4.21  ready in 92 ms
[T+0009] 
[T+0009]   ➜  Local:   http://127.0.0.1:44424/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 4.601s
```

#### Step Story

Proof of life errors detected.

#### Runner Output

```text
[T+0009] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0009] [2026-02-17T19:29:05.690-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /mnt/c/Program Files/Google/Chrome/Application/chrome.exe [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-port-44834 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080/?test=true]
```

#### Browser Logs

```text
[T+0013] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0013] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 776ms
```

#### Step Story

Dev server serves latest UI.

#### Runner Output

```text
[T+0013] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0014] $ vite --host "127.0.0.1" --port "45164"
[T+0014] 
[T+0014]   VITE v5.4.21  ready in 97 ms
[T+0014] 
[T+0014]   ➜  Local:   http://127.0.0.1:45164/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 163ms
section: hero
```

#### Step Story

Hero section and canvas validated visible.

#### Runner Output

```text
[T+0014] [TEST] RUN   06 Hero Section Validation
[T+0014]    [STEP] Waiting for Hero Section...
[T+0014] [WAIT] label=Hero Section detail=Hero section visibility
[T+0014]    [STEP] Waiting for Hero Canvas...
[T+0014] [WAIT] label=Hero Canvas detail=Hero canvas visibility
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 343ms
section: docs
```

#### Step Story

Docs section navigation and content validated.

#### Runner Output

```text
[T+0014] [TEST] RUN   07 Docs Section Validation
[T+0014]    [STEP] Navigating to Docs Section...
[T+0014]    [STEP] Waiting for Docs Content...
[T+0014] [WAIT] label=Docs Content detail=Docs content visibility
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 373ms
section: table
```

#### Step Story

Table section validated with data rows.

#### Runner Output

```text
[T+0015] [TEST] RUN   08 Table Section Validation
[T+0015]    [STEP] Navigating to Table Section...
[T+0015]    [STEP] Waiting for Robot Table...
[T+0015] [WAIT] label=Robot Table detail=table visibility
[T+0015]    [STEP] Waiting for data-ready=true...
[T+0015] [WAIT] label=Robot Table detail=table ready
[T+0015]    [STEP] Waiting for table rows...
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 433ms
section: three
```

#### Step Story

Three section navigation and canvas validated.

#### Runner Output

```text
[T+0015] [TEST] RUN   09 Three Section Validation
[T+0015]    [STEP] Navigating to Three Section...
[T+0015]    [STEP] Waiting for Three Canvas...
[T+0015] [WAIT] label=Three Canvas detail=Three canvas visibility
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 397ms
section: xterm
```

#### Step Story

Xterm section validated with command execution.

#### Runner Output

```text
[T+0015] [TEST] RUN   10 Xterm Section Validation
[T+0015]    [STEP] Navigating to Xterm Section...
[T+0016]    [STEP] Waiting for Xterm Terminal...
[T+0016] [WAIT] label=Xterm Terminal detail=terminal visibility
[T+0016]    [STEP] Waiting for data-ready=true...
[T+0016] [WAIT] label=Xterm Terminal detail=terminal ready
[T+0016]    [STEP] Waiting for Xterm Input...
[T+0016] [WAIT] label=Xterm Input detail=input visibility
[T+0016] [CLICK] kind=type target=Xterm Input detail=typing command; text="status --verbose"
[T+0016]    [STEP] Waiting for command echo...
[T+0016] [WAIT] label=Xterm Terminal detail=command echo check
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 515ms
section: video
```

#### Step Story

Video section validated with playback.

#### Runner Output

```text
[T+0016] [TEST] RUN   11 Video Section Validation
[T+0016]    [STEP] Navigating to Video Section...
[T+0016]    [STEP] Waiting for video playback (data-playing=true)...
[T+0016] [WAIT] label=Video Section detail=video playing
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.521s
```

#### Step Story

Lifecycle invariants maintained.

#### Runner Output

```text
[T+0016] [TEST] RUN   12 Lifecycle / Invariants
[T+0016]    [STEP] Checking section lifecycle...
[T+0018]    [STEP] Checking console logs for lifecycle tokens...
[T+0018]    [STEP] Checking active section count...
```

### 13 Menu Navigation Validation

```text
result: PASS
duration: 713ms
```

#### Step Story

Menu navigation flow validated.

#### Runner Output

```text
[T+0018] [TEST] RUN   13 Menu Navigation Validation
[T+0018] [CLICK] kind=aria target=Toggle Global Menu detail=open menu
[T+0018] [WAIT] label=Global Menu Panel detail=menu visible
[T+0018] [CLICK] kind=aria target=Navigate Telemetry detail=nav to telemetry
[T+0018]    [STEP] Waiting for menu to close and Telemetry to activate...
```

![13 Menu Navigation Validation sequence](../screenshots/menu_nav_grid.png)

### 14 Cleanup Verification

```text
result: PASS
duration: 240ms
```

#### Step Story

Cleanup successful.

#### Runner Output

```text
[T+0019] [TEST] RUN   14 Cleanup Verification
[T+0019] Cleaning up Dialtone-related process on port 8080 (PID: 443292, Name: main)...
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
