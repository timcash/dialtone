# Template Plugin Smoke Test Report

**Generated at:** Tue, 10 Feb 2026 13:34:38 PST

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

Checked 20 installs across 67 packages (no changes) [24.00ms]
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
dist/index.html                   3.68 kB │ gzip:   1.24 kB
dist/assets/index-EOT71fHf.css    3.93 kB │ gzip:   1.36 kB
dist/assets/index-mz4v2kC-.js     0.28 kB │ gzip:   0.22 kB
dist/assets/index-BTEGA10S.js     0.29 kB │ gzip:   0.22 kB
dist/assets/index-2JlVzRXj.js     0.50 kB │ gzip:   0.36 kB
dist/assets/Typing-BI9S19x9.js    0.56 kB │ gzip:   0.34 kB
dist/assets/index-DYXTFHjU.js     9.25 kB │ gzip:   3.26 kB
dist/assets/index-DIXToYof.js   468.08 kB │ gzip: 118.34 kB
✓ built in 622ms
```

---

## 4. UI & Interactivity

### 1. Hero Section Validation: PASS ✅

![Hero Section Validation](smoke_step_1.png)

---

### 2. Documentation Section Validation: PASS ✅

![Documentation Section Validation](smoke_step_2.png)

**Console Logs:**
```text
[log] "[SectionManager] 🚀 RESUME #docs"
[log] "[SectionManager] 💤 PAUSE #home"
[log] "[hero-viz] SLEEP"
```

---

### 3. Table Section Validation: PASS ✅

![Table Section Validation](smoke_step_3.png)

**Console Logs:**
```text
[log] "[SectionManager] 🚀 RESUME #table"
[log] "[SectionManager] 💤 PAUSE #docs"
```

---

### 4. Verify Header Hidden on Table: PASS ✅

![Verify Header Hidden on Table](smoke_step_4.png)

---

### 5. Settings Section Validation: PASS ✅

![Settings Section Validation](smoke_step_5.png)

**Console Logs:**
```text
[log] "[SectionManager] 🚀 RESUME #settings"
[log] "[SectionManager] 💤 PAUSE #table"
```

---

### 6. Return Home: PASS ✅

![Return Home](smoke_step_6.png)

**Console Logs:**
```text
[log] "[SectionManager] 🚀 RESUME #table"
[log] "[SectionManager] 💤 PAUSE #settings"
[log] "[SectionManager] 🚀 RESUME #docs"
[log] "[SectionManager] 💤 PAUSE #table"
[log] "[SectionManager] 🚀 RESUME #home"
[log] "[hero-viz] AWAKE"
```

---
