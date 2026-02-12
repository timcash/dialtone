# Template Plugin v3 Test Report

**Generated at:** Thu, 12 Feb 2026 00:00:00 PST
**Version:** `src_v3`
**Runner:** `test_v2`
**Status:** ✅ PASS

## 1. Preflight: Go + UI Checks

### Go Format: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone/src/plugins/template/src_v3
[cmd] go fmt ./...
[elapsed] 41ms
```

### Go Vet: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone/src/plugins/template/src_v3
[cmd] go vet ./...
[elapsed] 162ms
```

### Go Build: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone/src/plugins/template/src_v3
[cmd] go build ./...
[elapsed] 980ms
```

### UI Lint: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh bun exec --cwd /Users/dev/code/dialtone/src/plugins/template/src_v3/ui run lint
[elapsed] 641ms
```

### UI Format: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh bun exec --cwd /Users/dev/code/dialtone/src/plugins/template/src_v3/ui run format
[elapsed] 518ms
```

### UI Build: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh bun exec --cwd /Users/dev/code/dialtone/src/plugins/template/src_v3/ui run build
[elapsed] 1.304s

$ vite build
vite v5.4.21 building for production...
transforming...
✓ 24 modules transformed.
rendering chunks...
computing gzip size...
dist/index.html                   3.01 kB │ gzip:   1.08 kB
dist/assets/index-1a2b3c4d.css    6.10 kB │ gzip:   1.90 kB
dist/assets/index-5e6f7g8h.js     0.56 kB │ gzip:   0.36 kB
dist/assets/index-9i0j1k2l.js    11.25 kB │ gzip:   3.88 kB
dist/assets/index-3m4n5o6p.js   482.12 kB │ gzip: 121.44 kB
✓ built in 422ms
```

### Go Run: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone/src/plugins/template/src_v3
[cmd] go run cmd/main.go

Template Server starting on http://localhost:8080

[probe-ready] heartbeat ok in 4.112s
```

### UI Run: ✅ PASSED

```text
[dir] /Users/dev/code/dialtone
[cmd] /Users/dev/code/dialtone/dialtone.sh bun exec --cwd /Users/dev/code/dialtone/src/plugins/template/src_v3/ui run dev --host 127.0.0.1 --port 63937

$ vite --host "127.0.0.1" --port "63937"
VITE v5.4.21  ready in 88 ms

➜  Local:   http://127.0.0.1:63937/

[probe-ready] heartbeat ok in 4.307s
```

---

## 2. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
| error | "[PROOFOFLIFE] Intentional Browser Test Error" | ✅ CAPTURED |
| error | "[PROOFOFLIFE] Intentional Go Test Error" | ✅ CAPTURED |

---

## 3. Dev Server Test

### Dev Server Running (latest UI): PASS ✅

**Logs:**
```text
[log] "[dev] server start :63937 (debug)"
[log] "[dev] ui build hash=latest"
[log] "[dev] chromedp attached"
```

**Assertions:**
- [OK] dev server reachable
- [OK] UI reflects latest build

---

## 4. UI & Interactivity

### Lifecycle Verification Summary

| Event | Status | Description |
|---|---|---|
| LOADING | ✅ CAPTURED | Section chunk fetching initiated |
| LOADED | ✅ CAPTURED | Section code loaded into memory |
| START | ✅ CAPTURED | Section component initialized |
| RESUME | ✅ CAPTURED | Section active and visible |
| PAUSE | ✅ CAPTURED | Section suspended when hidden |

### 1. Hero Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 INITIAL LOAD #hero"
[log] "[SectionManager] 📦 LOADING #hero..."
[log] "[SectionManager] ✅ LOADED #hero (38ms)"
[log] "[SectionManager] ✨ START #hero"
[log] "[SectionManager] 🚀 RESUME #hero"
```

**Assertions:**
- [OK] aria-label="Hero Section" visible
- [OK] aria-label="Hero Canvas" present

![Hero Section Validation](test_step_1.png)

---

### 2. Docs Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #docs"
[log] "[SectionManager] 🧭 NAVIGATE TO #docs"
[log] "[SectionManager] 🚀 RESUME #docs"
[log] "[SectionManager] 💤 PAUSE #hero"
```

**Assertions:**
- [OK] aria-label="Docs Section" visible
- [OK] aria-label="Docs Header" visible

![Docs Section Validation](test_step_2.png)

---

### 3. Table Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #table"
[log] "[SectionManager] 🧭 NAVIGATE TO #table"
[log] "[SectionManager] 🚀 RESUME #table"
[log] "[SectionManager] 💤 PAUSE #docs"
```

**Assertions:**
- [OK] aria-label="Table Section" visible
- [OK] aria-label="Table Pagination Next" click
- [OK] aria-label="Table Pagination Prev" click

![Table Section Validation](test_step_3.png)

---

### 4. Three Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #three"
[log] "[SectionManager] 🧭 NAVIGATE TO #three"
[log] "[SectionManager] 🚀 RESUME #three"
[log] "[SectionManager] 💤 PAUSE #table"
```

**Assertions:**
- [OK] aria-label="Three Section" visible
- [OK] aria-label="Three Canvas" receives scroll

![Three Section Validation](test_step_4.png)

---

### 5. Xterm Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #xterm"
[log] "[SectionManager] 🧭 NAVIGATE TO #xterm"
[log] "[SectionManager] 🚀 RESUME #xterm"
[log] "[SectionManager] 💤 PAUSE #three"
```

**Assertions:**
- [OK] aria-label="Xterm Section" visible
- [OK] aria-label="Xterm Terminal" ready

![Xterm Section Validation](test_step_5.png)

---

### 6. Video Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #video"
[log] "[SectionManager] 🧭 NAVIGATE TO #video"
[log] "[SectionManager] 🚀 RESUME #video"
[log] "[SectionManager] 💤 PAUSE #xterm"
```

**Assertions:**
- [OK] aria-label="Video Section" visible
- [OK] aria-label="Test Video" playing

![Video Section Validation](test_step_6.png)

---

## 5. Lifecycle / Invariants

| Check | Status |
|---|---|
| Single active section | ✅ PASS |
| Active section is visible | ✅ PASS |
| Resume after load | ✅ PASS |

---

## 6. Cleanup Verification

| Check | Status |
|---|---|
| All Dialtone Chrome test processes exited | ✅ PASS |
| Chromedp session closed | ✅ PASS |
| Server process terminated | ✅ PASS |

---

## 7. Artifacts

- `test.log`
- `error.log`
- `test_step_1.png`
- `test_step_2.png`
- `test_step_3.png`
- `test_step_4.png`
- `test_step_5.png`
- `test_step_6.png`
