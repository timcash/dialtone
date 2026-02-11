# Template Plugin Smoke Test Report

**Generated at:** Tue, 10 Feb 2026 16:35:24 PST

## 1. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
| error | "[PROOFOFLIFE] Intentional Browser Test Error" | ✅ CAPTURED |
| error | [PROOFOFLIFE] Intentional Go Test Error | ✅ CAPTURED |

---

## 2. Real Errors & Warnings

✅ No actual issues detected.

---

## 3. Preflight: Environment & Build

### Install: ✅ PASSED

```text
bun install v1.2.22 (6bafe260)

Checked 20 installs across 67 packages (no changes) [32.00ms]
```

### Lint: ✅ PASSED

```text
$ tsc --noEmit
```

### Build: ✅ PASSED

```text
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
✓ built in 617ms
```

---

## 4. UI & Interactivity

### Lifecycle Verification Summary

| Event | Status | Description |
|---|---|---|
| LOADING | ✅ CAPTURED | Section chunk fetching initiated |
| LOADED | ✅ CAPTURED | Section code loaded into memory |
| START | ✅ CAPTURED | Section component initialized |
| RESUME / AWAKE | ✅ CAPTURED | Animation loop active and visible |
| PAUSE / SLEEP | ✅ CAPTURED | Animation loop suspended when off-screen |


### 1. Hero Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 INITIAL LOAD #home"
[log] "[SectionManager] 📦 LOADING #home..."
[log] "[SectionManager] ✅ LOADED #home (37ms)"
[log] "[SectionManager] ✨ START #home"
[log] "[hero-viz] SLEEP"
[error] "[PROOFOFLIFE] Intentional Browser Test Error"
[error] [PROOFOFLIFE] Intentional Go Test Error
[log] "[SectionManager] 📦 LOADING #docs..."
[log] "[SectionManager] 🚀 RESUME #home"
[log] "[hero-viz] AWAKE"
[log] "[SectionManager] ✅ LOADED #docs (3ms)"
[log] "[SectionManager] ✨ START #docs"
[log] "[docs-viz] SLEEP"
```

![Hero Section Validation](smoke_step_1.png)

---

### 2. Documentation Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #docs"
[log] "[SectionManager] 📦 LOADING #table..."
[log] "[SectionManager] 🚀 RESUME #docs"
[log] "[docs-viz] AWAKE"
[log] "[SectionManager] ✅ LOADED #table (2ms)"
[log] "[SectionManager] ✨ START #table"
[log] "[table-viz] SLEEP"
[log] "[SectionManager] 💤 PAUSE #home"
[log] "[hero-viz] SLEEP"
```

![Documentation Section Validation](smoke_step_2.png)

---

### 3. Table Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #table"
[log] "[SectionManager] 📦 LOADING #settings..."
[log] "[SectionManager] 🚀 RESUME #table"
[log] "[table-viz] AWAKE"
[log] "[SectionManager] ✅ LOADED #settings (2ms)"
[log] "[SectionManager] ✨ START #settings"
[log] "[settings-viz] SLEEP"
[log] "[SectionManager] 💤 PAUSE #docs"
[log] "[docs-viz] SLEEP"
```

![Table Section Validation](smoke_step_3.png)

---

### 4. Verify Header Hidden on Table: PASS ✅

![Verify Header Hidden on Table](smoke_step_4.png)

---

### 5. Settings Section Validation: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #settings"
[log] "[SectionManager] 🚀 RESUME #settings"
[log] "[settings-viz] AWAKE"
[log] "[SectionManager] 💤 PAUSE #table"
[log] "[table-viz] SLEEP"
```

![Settings Section Validation](smoke_step_5.png)

---

### 6. Return Home: PASS ✅

**Console Logs:**
```text
[log] "[SectionManager] 🧭 NAVIGATING TO #home"
[log] "[SectionManager] 🚀 RESUME #table"
[log] "[table-viz] AWAKE"
[log] "[SectionManager] 💤 PAUSE #settings"
[log] "[settings-viz] SLEEP"
[log] "[SectionManager] 🚀 RESUME #docs"
[log] "[docs-viz] AWAKE"
[log] "[SectionManager] 💤 PAUSE #table"
[log] "[table-viz] SLEEP"
[log] "[SectionManager] 🚀 RESUME #home"
[log] "[hero-viz] AWAKE"
```

![Return Home](smoke_step_6.png)

---
