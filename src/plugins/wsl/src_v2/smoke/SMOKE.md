# WSL Plugin Smoke Test Report

**Generated at:** Wed, 11 Feb 2026 13:29:38 PST

## 1. Preflight: Go + TypeScript/JavaScript Checks

### Go Format: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] go fmt ./...
[elapsed] 266ms
```

### Go Lint: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] go vet ./...
[elapsed] 408ms
```

### Go Build: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] go build ./...
[elapsed] 962ms
```

### UI Install: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui install --force
[elapsed] 2.549s

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui install --force
bun install v1.3.9 (cf6cdbbb)
Saved lockfile

+ @types/three@0.182.0
+ three@0.182.0
+ typescript@5.9.3
+ vite@5.4.21
+ xterm@5.3.0
+ xterm-addon-fit@0.8.0

23 packages installed [1.98s]
```

### UI TypeScript Lint: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run lint
[elapsed] 1.234s

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run lint
$ tsc --noEmit
```

### UI Build: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run build
[elapsed] 1.652s

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run build
$ vite build
[36mvite v5.4.21 [32mbuilding for production...[36m[39m
transforming...
[32m✓[39m 21 modules transformed.
rendering chunks...
computing gzip size...
[2mdist/[22m[32mindex.html                 [39m[1m[2m  3.35 kB[22m[1m[22m[2m │ gzip:   1.14 kB[22m
[2mdist/[22m[35massets/index-DwNj4dmR.css  [39m[1m[2m  6.55 kB[22m[1m[22m[2m │ gzip:   2.00 kB[22m
[2mdist/[22m[36massets/index-C_pF3mQv.js   [39m[1m[2m  0.17 kB[22m[1m[22m[2m │ gzip:   0.16 kB[22m
[2mdist/[22m[36massets/index-DanylEBI.js   [39m[1m[2m  0.40 kB[22m[1m[22m[2m │ gzip:   0.31 kB[22m
[2mdist/[22m[36massets/Typing-BI9S19x9.js  [39m[1m[2m  0.56 kB[22m[1m[22m[2m │ gzip:   0.34 kB[22m
[2mdist/[22m[36massets/index-DP1acllJ.js   [39m[1m[2m  4.34 kB[22m[1m[22m[2m │ gzip:   1.75 kB[22m
[2mdist/[22m[36massets/index-BCgPC4L8.js   [39m[1m[2m 12.40 kB[22m[1m[22m[2m │ gzip:   4.17 kB[22m
[2mdist/[22m[36massets/index-0jkOUv2m.js   [39m[1m[2m485.79 kB[22m[1m[22m[2m │ gzip: 123.97 kB[22m
[32m✓ built in 806ms[39m
```

### Source Prettier Format (JS/TS): ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2 x prettier --write ui\src\components\docs\index.ts ui\src\components\home\index.ts ui\src\components\settings\index.ts ui\src\components\table\index.ts ui\src\dialtone-ui.ts ui\src\main.ts ui\src\vite-env.d.ts ui\vite.config.ts

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2 x prettier --write ui\src\components\docs\index.ts ui\src\components\home\index.ts ui\src\components\settings\index.ts ui\src\components\table\index.ts ui\src\dialtone-ui.ts ui\src\main.ts ui\src\vite-env.d.ts ui\vite.config.ts
[90mui/src/components/docs/index.ts[39m 33ms (unchanged)
[90mui/src/components/home/index.ts[39m 17ms (unchanged)
[90mui/src/components/settings/index.ts[39m 2ms (unchanged)
[90mui/src/components/table/index.ts[39m 22ms (unchanged)
[90mui/src/dialtone-ui.ts[39m 1ms (unchanged)
[90mui/src/main.ts[39m 7ms (unchanged)
[90mui/src/vite-env.d.ts[39m 2ms (unchanged)
ui/vite.config.ts 2ms
```

### Source Prettier Lint (JS/TS): ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2 x prettier --check ui\src\components\docs\index.ts ui\src\components\home\index.ts ui\src\components\settings\index.ts ui\src\components\table\index.ts ui\src\dialtone-ui.ts ui\src\main.ts ui\src\vite-env.d.ts ui\vite.config.ts

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2 x prettier --check ui\src\components\docs\index.ts ui\src\components\home\index.ts ui\src\components\settings\index.ts ui\src\components\table\index.ts ui\src\dialtone-ui.ts ui\src\main.ts ui\src\vite-env.d.ts ui\vite.config.ts
Checking formatting...
All matched files use Prettier code style!
```

### Go Run: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2
[cmd] go run cmd/main.go


[probe-ready] port 8080 became reachable in 3ms
```

### UI Run: ✅ PASSED

```text
[dir] C:\Users\timca\code3\dialtone
[cmd] powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\Users\timca\code3\dialtone\dialtone.ps1 bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run dev --host 127.0.0.1 --port 52181

Running: C:\Users\timca\dialtone_dependencies\go\bin\go.exe run src/cmd/dev/main.go bun exec --cwd C:\Users\timca\code3\dialtone\src\plugins\wsl\src_v2\ui run dev --host 127.0.0.1 --port 52181
$ vite --host "127.0.0.1" --port "52181"

  [32m[1mVITE[22m v5.4.21[39m  [2mready in [0m[1m196[22m[2m[0m ms[22m

  [32m➜[39m  [1mLocal[22m:   [36mhttp://127.0.0.1:[1m52181[22m/[39m

[probe-warning] timed out waiting for UI Run process shutdown
[probe-ready] port 52181 became reachable in 6.002s
```

---

## 2. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
| error | "[PROOFOFLIFE] Intentional Browser Test Error" | ✅ CAPTURED |
| error | [PROOFOFLIFE] Intentional Go Test Error | ✅ CAPTURED |

---

## 3. Real Errors & Warnings

✅ No actual issues detected.

---

## 4. UI & Interactivity

### Lifecycle Verification Summary

| Event | Status | Description |
|---|---|---|
| LOADING | ❌ MISSING | Section chunk fetching initiated |
| LOADED | ❌ MISSING | Section code loaded into memory |
| START | ❌ MISSING | Section component initialized |
| RESUME / AWAKE | ❌ MISSING | Animation loop active and visible |
| PAUSE / SLEEP | ❌ MISSING | Animation loop suspended when off-screen |


### 1. Home Section Validation: PASS ✅

**Console Logs:**
```text
[error] "[PROOFOFLIFE] Intentional Browser Test Error"
```

![Home Section Validation](smoke_step_1.png)

---

### 2. Documentation Section Validation: PASS ✅

![Documentation Section Validation](smoke_step_2.png)

---

### 3. Table Section Validation: PASS ✅

![Table Section Validation](smoke_step_3.png)

---
