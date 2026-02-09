# Nix Final Smoke Test Report

**Generated:** Mon, 09 Feb 2026 15:38:18 PST

### Test 1: 1. Verify Browser Error Capture

- **Status:** ✅ PASSED
- **Error Log:** `"[SMOKE-ERR-FINAL] Manual Capture Check"`

**Stack Trace:**
```
   (:0:8)

```

#### Visual proof
![Step 1](smoke_step_0.png)

#### Last 20 Console Logs
```
[error] "[SMOKE-ERR-FINAL] Manual Capture Check"
[log] "[main] 🧭 Navigating to: #s-viz"
```

---

### Test 2: 2. Hero Section Validation

- **Status:** ✅ PASSED

#### Visual proof
![Step 2](smoke_step_1.png)

#### Last 20 Console Logs
```
[error] "[SMOKE-ERR-FINAL] Manual Capture Check"
[log] "[main] 🧭 Navigating to: #s-viz"
```

---

### Test 3: 3. Spawn Two Nix Nodes

- **Status:** ✅ PASSED

#### Visual proof
![Step 3](smoke_step_2.png)

#### Last 20 Console Logs
```
[error] "[SMOKE-ERR-FINAL] Manual Capture Check"
[log] "[main] 🧭 Navigating to: #s-viz"
[log] "[main] 🧭 Navigating to: #s-nixtable"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
```

---

### Test 4: 4. Terminate proc-1 via Aria Label

- **Status:** ❌ FAILED
- **Error Log:** `manual step timeout (5s)`

#### Visual proof
![Step 4](smoke_step_3.png)

#### Last 20 Console Logs
```
[error] "[SMOKE-ERR-FINAL] Manual Capture Check"
[log] "[main] 🧭 Navigating to: #s-viz"
[log] "[main] 🧭 Navigating to: #s-nixtable"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
[log] "[NIX] Requesting stop for node proc-1"
[log] "[NIX] Stop command acknowledged for proc-1"
```

---

### Test 5: 5. Verify proc-2 Persistence

- **Status:** ✅ PASSED

#### Visual proof
![Step 5](smoke_step_4.png)

#### Last 20 Console Logs
```
[error] "[SMOKE-ERR-FINAL] Manual Capture Check"
[log] "[main] 🧭 Navigating to: #s-viz"
[log] "[main] 🧭 Navigating to: #s-nixtable"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
[log] "[NIX] Requesting stop for node proc-1"
[log] "[NIX] Stop command acknowledged for proc-1"
```

---