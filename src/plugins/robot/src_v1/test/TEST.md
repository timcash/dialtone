# Robot Plugin src_v1 Test Report

**Generated at:** Mon, 16 Feb 2026 17:23:55 -0800
**Version:** `src_v1`
**Runner:** `test_v2`
**Status:** ✅ PASS
**Total Time:** `25.534s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| 01 Preflight (Go/UI) | ✅ PASS | `11.167s` |
| 02 Go Run (Mock Server Check) | ✅ PASS | `3.628s` |
| 03 UI Run | ✅ PASS | `779ms` |
| 04 Expected Errors (Proof of Life) | ✅ PASS | `4.435s` |
| 05 Dev Server Running (latest UI) | ✅ PASS | `778ms` |
| 06 Hero Section Validation | ✅ PASS | `138ms` |
| 07 Docs Section Validation | ✅ PASS | `343ms` |
| 08 Table Section Validation | ✅ PASS | `378ms` |
| 09 Three Section Validation | ✅ PASS | `376ms` |
| 10 Xterm Section Validation | ✅ PASS | `384ms` |
| 11 Video Section Validation | ✅ PASS | `481ms` |
| 12 Lifecycle / Invariants | ✅ PASS | `1.787s` |
| 13 Menu Navigation Validation | ✅ PASS | `353ms` |
| 14 Cleanup Verification | ✅ PASS | `401ms` |

## Step Logs

### 01 Preflight (Go/UI)

```text
result: PASS
duration: 11.167s
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
[T+0001] [2026-02-16T17:23:32.792-08:00 | INFO | go.go:RunGo:33] Running: go [fmt ./src/plugins/robot/src_v1/...]
[T+0001] >> [Robot] Vet: src_v1
[T+0001] [2026-02-16T17:23:33.430-08:00 | INFO | go.go:RunGo:33] Running: go [vet ./src/plugins/robot/src_v1/...]
[T+0002] >> [Robot] Go Build: src_v1
[T+0002] [2026-02-16T17:23:34.161-08:00 | INFO | go.go:RunGo:33] Running: go [build ./src/plugins/robot/src_v1/...]
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
[T+0006] dist/index.html                   4.88 kB │ gzip:   1.31 kB
[T+0006] dist/assets/index-6GBZ9nXN.css    5.24 kB │ gzip:   1.92 kB
[T+0006] dist/assets/index-ClTVjYjc.css   10.22 kB │ gzip:   2.68 kB
[T+0006] dist/assets/index-Dbd9b_e5.js     0.08 kB │ gzip:   0.10 kB
[T+0006] dist/assets/index-DP7yMgmI.js     0.79 kB │ gzip:   0.44 kB
[T+0006] dist/assets/index--IuBKJNs.js     1.10 kB │ gzip:   0.53 kB
[T+0006] dist/assets/index-BTTHdoMO.js     1.17 kB │ gzip:   0.65 kB
[T+0006] dist/assets/index-EfMFG-NX.js     2.94 kB │ gzip:   1.13 kB
[T+0006] dist/assets/index-DHaAbNI8.js   192.98 kB │ gzip:  60.24 kB
[T+0006] dist/assets/index-CP9Roacu.js   334.99 kB │ gzip:  85.16 kB
[T+0006] dist/assets/index-Dm2IEHks.js   497.97 kB │ gzip: 126.50 kB
[T+0006] ✓ built in 1.23s
[T+0006] >> [Robot] Building Dialtone Binary into src/plugins/robot/bin
[T+0007] [2026-02-16T17:23:38.672-08:00 | INFO | build.go:RunBuild:110] Building Dialtone for Linux amd64 using Podman (gcc, g++)...
[T+0007] [2026-02-16T17:23:39.045-08:00 | INFO | build.go:RunBuild:110] Using optimized 'dialtone-builder' image (skipping apt-get install)
[T+0007] [2026-02-16T17:23:39.045-08:00 | INFO | build.go:RunBuild:110] Running: podman [run --rm -v /home/user/dialtone:/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=amd64 -e CGO_ENABLED=1 -e CC=gcc -e CXX=g++ dialtone-builder bash -c go build -buildvcs=false -ldflags="-s -w" -trimpath -tags no_duckdb -o src/plugins/robot/bin/dialtone-amd64 src/cmd/dialtone/main.go]
[T+0011] [2026-02-16T17:23:42.811-08:00 | INFO | build.go:RunBuild:110] Build successful: bin/dialtone-amd64
```

### 02 Go Run (Mock Server Check)

```text
result: PASS
duration: 3.628s
```

#### Runner Output

```text
[T+0011] [TEST] RUN   02 Go Run (Mock Server Check)
[T+0011] Cleaning up stale Linux process on port 8080 (PID: 257825) via lsof...
[T+0014] [2026-02-16T17:23:46.242-08:00 | INFO | robot.go:RunRobot:69] [WARNING] Process is not running as a systemd service. Consider running via systemctl.
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:RunStart:226] NATS server started on port 4222 (local only)
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:runLocalOnly:248] NATS WS proxy ports: external=4223 internal=4223
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:runLocalOnly:248] Using provided static web assets
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:runLocalOnly:250] NATS WS proxy ports: external=4223 internal=4223
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:runLocalOnly:250] Using provided static web assets
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | robot.go:RunStart:226] Web UI (Local Only): Serving at http://0.0.0.0:8080
[T+0014] [2026-02-16T17:23:46.398-08:00 | INFO | asm_amd64.s:goexit:1693] Starting Mock Mavlink Publisher...
```

### 03 UI Run

```text
result: PASS
duration: 779ms
```

#### Runner Output

```text
[T+0014] [TEST] RUN   03 UI Run
[T+0015] >> [Robot] UI Run: src_v1
[T+0015] $ vite --host "127.0.0.1" --port "43802"
[T+0015] 
[T+0015]   VITE v5.4.21  ready in 93 ms
[T+0015] 
[T+0015]   ➜  Local:   http://127.0.0.1:43802/
```

### 04 Expected Errors (Proof of Life)

```text
result: PASS
duration: 4.435s
```

#### Runner Output

```text
[T+0015] [TEST] RUN   04 Expected Errors (Proof of Life)
[T+0015] [2026-02-16T17:23:47.257-08:00 | INFO | chrome.go:StartSession:179] DEBUG: Launching Chrome: /mnt/c/Program Files/Google/Chrome/Application/chrome.exe [--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:\Users\timca\AppData\Local\Temp\dialtone-chrome-test-port-44922 --new-window --dialtone-origin=true --dialtone-role=test --headless=new http://127.0.0.1:8080]
```

#### Browser Logs

```text
[T+0020] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

#### Browser Errors

```text
[T+0020] [error] [PROOFOFLIFE] Intentional Browser Test Error
```

### 05 Dev Server Running (latest UI)

```text
result: PASS
duration: 778ms
```

#### Runner Output

```text
[T+0020] [TEST] RUN   05 Dev Server Running (latest UI)
[T+0020] >> [Robot] UI Run: src_v1
[T+0020] $ vite --host "127.0.0.1" --port "44880"
[T+0020] 
[T+0020]   VITE v5.4.21  ready in 100 ms
[T+0020] 
[T+0020]   ➜  Local:   http://127.0.0.1:44880/
```

### 06 Hero Section Validation

```text
result: PASS
duration: 138ms
section: hero
```

#### Runner Output

```text
[T+0020] [TEST] RUN   06 Hero Section Validation
```

![06 Hero Section Validation](../screenshots/test_step_1.png)

### 07 Docs Section Validation

```text
result: PASS
duration: 343ms
section: docs
```

#### Runner Output

```text
[T+0020] [TEST] RUN   07 Docs Section Validation
```

![07 Docs Section Validation](../screenshots/test_step_2.png)

### 08 Table Section Validation

```text
result: PASS
duration: 378ms
section: table
```

#### Runner Output

```text
[T+0021] [TEST] RUN   08 Table Section Validation
```

![08 Table Section Validation](../screenshots/test_step_3.png)

### 09 Three Section Validation

```text
result: PASS
duration: 376ms
section: three
```

#### Runner Output

```text
[T+0021] [TEST] RUN   09 Three Section Validation
```

![09 Three Section Validation](../screenshots/test_step_4.png)

### 10 Xterm Section Validation

```text
result: PASS
duration: 384ms
section: xterm
```

#### Runner Output

```text
[T+0022] [TEST] RUN   10 Xterm Section Validation
```

![10 Xterm Section Validation](../screenshots/test_step_5.png)

### 11 Video Section Validation

```text
result: PASS
duration: 481ms
section: video
```

#### Runner Output

```text
[T+0022] [TEST] RUN   11 Video Section Validation
```

![11 Video Section Validation](../screenshots/test_step_6.png)

### 12 Lifecycle / Invariants

```text
result: PASS
duration: 1.787s
```

#### Runner Output

```text
[T+0022] [TEST] RUN   12 Lifecycle / Invariants
```

### 13 Menu Navigation Validation

```text
result: PASS
duration: 353ms
```

#### Runner Output

```text
[T+0024] [TEST] RUN   13 Menu Navigation Validation
```

![13 Menu Navigation Validation sequence](../screenshots/menu_nav_grid.png)

### 14 Cleanup Verification

```text
result: PASS
duration: 401ms
```

#### Runner Output

```text
[T+0025] [TEST] RUN   14 Cleanup Verification
[T+0025] Cleaning up stale Linux process on port 8080 (PID: 260162) via lsof...
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
