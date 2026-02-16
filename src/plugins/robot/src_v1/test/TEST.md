# Robot Plugin src_v1 Test Report

**Generated at:** Mon, 16 Feb 2026 15:54:39 -0800
**Version:** `src_v1`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `26.441s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `12.104s` |
| 02 Go Run (Mock Server Check) | ✅ PASS | `3.897s` |
| 03 UI Run | ✅ PASS | `781ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `4.525s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `776ms` |
| 06 Hero Section Validation | ✅ PASS | `155ms` |
| 07 Docs Section Validation | ✅ PASS | `358ms` |
| 08 Table Section Validation | ✅ PASS | `365ms` |
| 09 Three Section Validation | ✅ PASS | `373ms` |
| 10 Xterm Section Validation | ✅ PASS | `372ms` |
| 11 Video Section Validation | ✅ PASS | `492ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.791s` |
| 13 Cleanup Verification | ✅ PASS | `449ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 12.104s
```

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
[T+0000] 26 packages installed [40.00ms]
[T+0000] >> [Robot] Fmt: src_v1
[T+0001] [2026-02-16T15:54:15.657-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/robot/src_v1/...]
[T+0001] >> [Robot] Vet: src_v1
[T+0001] [2026-02-16T15:54:16.304-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/robot/src_v1/...]
[T+0002] >> [Robot] Go Build: src_v1
[T+0002] [2026-02-16T15:54:17.027-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/robot/src_v1/...]
[T+0003] >> [Robot] Lint: src_v1
[T+0003] $ tsc --noEmit
[T+0004] >> [Robot] Format: src_v1
[T+0004] $ echo format-ok
[T+0004] format-ok
[T+0004] >> [Robot] Building UI: src_v1
[T+0005] $ vite build
[T+0005] vite v5.4.21 building for production...
[T+0005] transforming...
[T+0006] ✓ 22 modules transformed.
[T+0006] rendering chunks...
[T+0006] computing gzip size...
[T+0006] dist/index.html                   3.99 kB │ gzip:   1.13 kB
[T+0006] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0006] dist/assets/index-DwklqrbF.css    7.82 kB │ gzip:   2.26 kB
[T+0006] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0006] dist/assets/index-DP7yMgmI.js     0.79 kB │ gzip:   0.44 kB
[T+0006] dist/assets/index-WwFTIwNg.js     0.94 kB │ gzip:   0.44 kB
[T+0006] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0006] dist/assets/index-CP_Shkjd.js     2.32 kB │ gzip:   1.00 kB
[T+0006] dist/assets/index-DvhyHFmD.js    53.40 kB │ gzip:  19.05 kB
[T+0006] dist/assets/index-BFRwSQ3x.js   334.99 kB │ gzip:  85.16 kB
[T+0006] dist/assets/index-wozOi2x5.js   496.42 kB │ gzip: 126.01 kB
[T+0006] ✓ built in 1.09s
[T+0006] >> [Robot] Building Dialtone Binary into src/plugins/robot/bin
[T+0006] [2026-02-16T15:54:21.464-08:00 | INFO | build.go:RunBuild:110] Building Dialtone for Linux amd64 using Podman (gcc, g++)...
[T+0007] [2026-02-16T15:54:21.889-08:00 | INFO | build.go:RunBuild:110] Using optimized 'dialtone-builder' image (skipping apt-get install)
[T+0007] [2026-02-16T15:54:21.889-08:00 | INFO | build.go:RunBuild:110] Running: podman [run --rm -v /home/user/dialtone:/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 -e CC=gcc -e CXX=g++ dialtone-builder bash -c go build -buildvcs=false -o src/plugins/robot/bin/dialtone-amd64 src/cmd/dialtone/main.go]
[T+0012] [2026-02-16T15:54:24.678-08:00 | INFO | build.go:RunBuild:110] Build successful: bin/dialtone-amd64
```

### 02 Go Run (Mock Server Check)

```text
result: PASS
duration: 3.897s
```

#### Runner Output

```text
[T+0012] [TEST] RUN   02 Go Run (Mock Server Check)
[T+0015] [2026-02-16T15:54:28.247-08:00 | INFO | robot.go:RunRobot:62] [WARNING] Process is not running as a systemd service. Consider running via systemctl.
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:RunStart:212] NATS server started on port 4222 (local only)
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:runLocalOnly:234] NATS WS proxy ports: external=4223 internal=4223
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | asm_amd64.s:goexit:1693] Starting Mock Mavlink Publisher...
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:runLocalOnly:234] Using provided static web assets
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:runLocalOnly:236] NATS WS proxy ports: external=4223 internal=4223
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:runLocalOnly:236] Using provided static web assets
[T+0015] [2026-02-16T15:54:28.404-08:00 | INFO | robot.go:RunStart:212] Web UI (Local Only): Serving at http://0.0.0.0:8080
```

### 03 UI Run

```text
result: PASS
duration: 781ms
```

#### Runner Output

```text
[T+0016] [TEST] RUN   03 UI Run
[T+0016] >> [Robot] UI Run: src_v1
[T+0016] $ vite --host "127.0.0.1" --port "44132"
[T+0016] 
[T+0016]   VITE v5.4.21  ready in 99 ms
[T+0016] 
[T+0016]   ➜  Local:   http://127.0.0.1:44132/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 4.525s
```

#### Runner Output

```text
[T+0016] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0016] [2026-02-16T15:54:29.392-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /mnt/c/Program Files/Google/Chrome/Application/chrome.exe [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-port-44692 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080]
```

#### Browser Logs

```text
[T+0021] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0021] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 776ms
```

#### Runner Output

```text
[T+0021] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0021] >> [Robot] UI Run: src_v1
[T+0021] $ vite --host "127.0.0.1" --port "45280"
[T+0021] 
[T+0021]   VITE v5.4.21  ready in 107 ms
[T+0021] 
[T+0021]   ➜  Local:   http://127.0.0.1:45280/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 155ms
section: hero
```

#### Runner Output

```text
[T+0022] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 358ms
section: docs
```

#### Runner Output

```text
[T+0022] [TEST] RUN   07 Docs Section Validation
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 365ms
section: table
```

#### Runner Output

```text
[T+0022] [TEST] RUN   08 Table Section Validation
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 373ms
section: three
```

#### Runner Output

```text
[T+0022] [TEST] RUN   09 Three Section Validation
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 372ms
section: xterm
```

#### Runner Output

```text
[T+0023] [TEST] RUN   10 Xterm Section Validation
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
[T+0023] [TEST] RUN   11 Video Section Validation
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.791s
```

#### Runner Output

```text
[T+0024] [TEST] RUN   12 Lifecycle / Invariants
```

### 13 Cleanup Verification

```text
result: PASS
duration: 449ms
```

#### Runner Output

```text
[T+0025] [TEST] RUN   13 Cleanup Verification
[T+0026] Cleaning up stale Linux process on port 8080 (PID: 204838) via lsof...
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
