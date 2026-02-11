# Template Plugin Smoke Test Report

**Generated at:** Wed, 11 Feb 2026 10:32:46 PST

## 1. Preflight: Go + TypeScript/JavaScript Checks

### Go Format: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go fmt ./...
[elapsed] 45ms
```

### Go Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go vet ./...
[elapsed] 175ms
```

### Go Build: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go build ./...
[elapsed] 1.017s
```

### UI Install: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone
[cmd] /Users/tim/code/dialtone/dialtone.sh bun exec --cwd /Users/tim/code/dialtone/src/plugins/template/src_v2/ui install
[elapsed] 329ms

bun install v1.3.9 (cf6cdbbb)

Checked 20 installs across 67 packages (no changes) [2.00ms]
```

### UI TypeScript Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone
[cmd] /Users/tim/code/dialtone/dialtone.sh bun exec --cwd /Users/tim/code/dialtone/src/plugins/template/src_v2/ui run lint
[elapsed] 774ms

$ tsc --noEmit
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

