# WSL Robust Smoke Test Report

**Started:** Wed, 11 Feb 2026 10:56:54 PST

## Pre-test Verification

- [x] **Go Vet:** PASSED
- [x] **TypeScript Lint:** PASSED
- [x] **Vite Build:** PASSED
## Port Verification

- [x] **Port Detection:** PASSED (Found: 8087)

- [x] **Port Accessibility:** PASSED (Port 8087 active)

---
- [x] **Backend Status API:** PASSED (status=Online)
- [x] **Backend List API:** PASSED (2 instances)
- [x] **UI Serving:** PASSED (index.html served)
- [x] **Backend Invalid POST:** PASSED (accepted empty name)
- [x] **Backend Stop (empty):** PASSED (status=202)

---
## Browser Verification

- [x] **Debug Browser Attached:** ws://127.0.0.1:49908/devtools/browser/10f644d4-8178-4d41-8328-72802ede04a0
- [x] **Console Logging Enabled:** Active

---

### 1. Home Section Validation: PASSED

![1. Home Section Validation](smoke_step_0.png)

---

### 2. Documentation Section: PASSED

![2. Documentation Section](smoke_step_1.png)

---

### 3. WSL Table Rendering: PASSED

![3. WSL Table Rendering](smoke_step_2.png)

<details><summary>Console Logs (2)</summary>

```text
[log] "[WSL] TableSection mounting..."
[log] "[WSL] Connecting to WebSocket..."
```

</details>

---

### 4. Verify Header Hidden: PASSED

![4. Verify Header Hidden](smoke_step_3.png)

---

### 5. Spawn WSL Node: PASSED

![5. Spawn WSL Node](smoke_step_4.png)

<details><summary>Console Logs (1)</summary>

```text
[log] "[UI_ACTION] User requested SPAWN for: smoke-test-node"
```

</details>

---

### 6. Verify Running & Stats: PASSED

![6. Verify Running & Stats](smoke_step_5.png)

---

### 7. Stop Node: PASSED

![7. Stop Node](smoke_step_6.png)

<details><summary>Console Logs (1)</summary>

```text
[log] "[UI_ACTION] User clicked STOP for: smoke-test-node"
```

</details>

---

### 8. Delete Node: PASSED

![8. Delete Node](smoke_step_7.png)

<details><summary>Console Logs (1)</summary>

```text
[log] "[UI_ACTION] User clicked DELETE for: smoke-test-node"
```

</details>

---
