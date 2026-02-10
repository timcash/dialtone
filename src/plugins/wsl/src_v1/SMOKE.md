# WSL Robust Smoke Test Report

**Started:** Tue, 10 Feb 2026 09:38:21 PST

## Pre-test Verification

- [x] **Go Vet:** PASSED
- [ ] **TypeScript Lint:** FAILED
- [x] **Vite Build:** PASSED

---
## Browser Verification

- [x] **Debug Browser Attached:** ws://127.0.0.1:64211/devtools/browser/3ac361c5-99ee-468f-b745-4f23ff8e5ee9
- [x] **Console Logging Enabled:** Active

---

### 1. 1. Home Section Validation

![1. Home Section Validation](smoke_step_0.png)

---

### 2. 2. Documentation Section

![2. Documentation Section](smoke_step_1.png)

---

### 3. 3. WSL Table Rendering

![3. WSL Table Rendering](smoke_step_2.png)

<details><summary>Console Logs (2)</summary>

```text
[log] "[WSL] TableSection mounting..."
[log] "[WSL] Connecting to WebSocket..."
```

</details>

---

### 4. 4. Verify Header Hidden

![4. Verify Header Hidden](smoke_step_3.png)

---
