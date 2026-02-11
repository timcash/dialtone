# Template Plugin Smoke Test Report

**Generated at:** Wed, 11 Feb 2026 10:13:28 PST

## 1. Preflight: Go + TypeScript/JavaScript Checks

### Go Format: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go fmt ./...
[elapsed] 58ms
```

### Go Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go vet ./...
[elapsed] 166ms
```

### Go Build: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2
[cmd] go build ./...
[elapsed] 996ms
```

### UI Install: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun install
[elapsed] 5ms

bun install v1.2.23 (cf136713)

Checked 20 installs across 67 packages (no changes) [2.00ms]
```

### UI TypeScript Lint: ✅ PASSED

```text
[dir] /Users/tim/code/dialtone/src/plugins/template/src_v2/ui
[cmd] bun run lint
[elapsed] 363ms

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

