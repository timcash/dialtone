# Template Plugin Smoke Test Report

**Generated at:** Wed, 11 Feb 2026 09:25:26 PST

## 1. Preflight: Go + TypeScript/JavaScript Checks

### Go Format: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go fmt ./...
```

### Go Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go vet ./...
```

### Go Build: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go build ./...
```

### UI Install: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun install

bun install v1.2.23 (cf136713)

Checked 20 installs across 67 packages (no changes) [2.00ms]
```

### UI TypeScript Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun run lint

$ tsc --noEmit
```

### UI Build: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun run build

$ vite build
vite v5.4.21 building for production...
transforming...
✓ 17 modules transformed.
rendering chunks...
computing gzip size...
dist/index.html                   2.93 kB │ gzip:   1.05 kB
dist/assets/index-QymdLqyU.css    5.42 kB │ gzip:   1.74 kB
dist/assets/index-DLzIJ4mn.js     0.41 kB │ gzip:   0.29 kB
dist/assets/index-BA4wnujV.js     0.42 kB │ gzip:   0.29 kB
dist/assets/Typing-BI9S19x9.js    0.56 kB │ gzip:   0.34 kB
dist/assets/index-CnCedI4T.js     0.66 kB │ gzip:   0.43 kB
dist/assets/index-Cw6LpDXf.js     9.98 kB │ gzip:   3.50 kB
dist/assets/index-DYnRDjrK.js   468.29 kB │ gzip: 118.39 kB
✓ built in 436ms
```

### Source Prettier Check (JS/TS): ❌ FAILED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] bunx prettier --check ui/dist/assets/Typing-BI9S19x9.js ui/dist/assets/index-BA4wnujV.js ui/dist/assets/index-CnCedI4T.js ui/dist/assets/index-Cw6LpDXf.js ui/dist/assets/index-DLzIJ4mn.js ui/dist/assets/index-DYnRDjrK.js ui/src/components/docs/index.ts ui/src/components/home/index.ts ui/src/components/settings/index.ts ui/src/components/table/index.ts ui/src/main.ts ui/vite.config.ts

Checking formatting...
[warn] ui/dist/assets/Typing-BI9S19x9.js
[warn] ui/dist/assets/index-BA4wnujV.js
[warn] ui/dist/assets/index-CnCedI4T.js
[warn] ui/dist/assets/index-Cw6LpDXf.js
[warn] ui/dist/assets/index-DLzIJ4mn.js
[warn] ui/dist/assets/index-DYnRDjrK.js
[warn] ui/src/components/docs/index.ts
[warn] ui/src/components/home/index.ts
[warn] ui/src/components/settings/index.ts
[warn] ui/src/components/table/index.ts
[warn] ui/src/main.ts
[warn] ui/vite.config.ts
[warn] Code style issues found in 12 files. Run Prettier with --write to fix.
```

### Go Run: ❌ FAILED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go run cmd/main.go

Template Server starting on http://localhost:8080

[probe-error] timed out waiting for Go Run process shutdown
```

### UI Run: ❌ FAILED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun run dev --host 127.0.0.1 --port 62332

$ vite --host "127.0.0.1" --port "62332"

  VITE v5.4.21  ready in 85 ms

  ➜  Local:   http://127.0.0.1:62332/

[probe-error] timed out waiting for UI Run process shutdown
```

---

## 2. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
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

