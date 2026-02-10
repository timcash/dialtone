# Nix Robust Smoke Test Report

**Started:** Mon, 09 Feb 2026 16:50:56 PST

### 1. Verify Browser Error Capture

- **Status:** ✅ PASSED
- **Conditions:** Navigate to home and verify captured log

#### Visual proof
![1. Verify Browser Error Capture](smoke_step_0.png)

#### Last 10 Console Logs
```
[error] "[SMOKE-VERIFY-ERR] Log pipeline verified"
[log] "[main] 🧭 Navigating to: #nix-hero (snappy)"
```

---
### 2. Hero Section Validation

- **Status:** ✅ PASSED
- **Conditions:** Viz container and marketing overlay visible

#### Visual proof
![2. Hero Section Validation](smoke_step_1.png)

#### Last 10 Console Logs
```
[error] "[SMOKE-VERIFY-ERR] Log pipeline verified"
[log] "[main] 🧭 Navigating to: #nix-hero (snappy)"
[log] "[main] ✅ Snappy scroll settled for #nix-hero"
```

---
### 3. Documentation Section Validation

- **Status:** ✅ PASSED
- **Conditions:** Navigate to nix-docs and verify content

#### Visual proof
![3. Documentation Section Validation](smoke_step_2.png)

#### Last 10 Console Logs
```
[error] "[SMOKE-VERIFY-ERR] Log pipeline verified"
[log] "[main] 🧭 Navigating to: #nix-hero (snappy)"
[log] "[main] ✅ Snappy scroll settled for #nix-hero"
[log] "[main] 🧭 Navigating to: #nix-docs (snappy)"
```

---
### 4. Navigate to Nix Table and Verify Rendering

- **Status:** ✅ PASSED
- **Conditions:** Switch to nix-table and verify fullscreen layout + hidden header/menu

#### Visual proof
![4. Navigate to Nix Table and Verify Rendering](smoke_step_3.png)

#### Last 10 Console Logs
```
[error] "[SMOKE-VERIFY-ERR] Log pipeline verified"
[log] "[main] 🧭 Navigating to: #nix-hero (snappy)"
[log] "[main] ✅ Snappy scroll settled for #nix-hero"
[log] "[main] 🧭 Navigating to: #nix-docs (snappy)"
[log] "[main] 🧭 Navigating to: #nix-table (snappy)"
[log] "[NIX] nix-table setVisible:" false
[log] "[NIX] nix-table setVisible:" true
```

---
### 5. Spawn Two Nix Nodes

- **Status:** ✅ PASSED
- **Conditions:** Two nodes appear in table

#### Visual proof
![5. Spawn Two Nix Nodes](smoke_step_4.png)

#### Last 10 Console Logs
```
[log] "[main] 🧭 Navigating to: #nix-table (snappy)"
[log] "[NIX] nix-table setVisible:" false
[log] "[NIX] nix-table setVisible:" true
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Processes updated:" "proc-1:running"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
[log] "[NIX] Processes updated:" "proc-1:running, proc-2:running"
[log] "[main] ✅ Snappy scroll settled for #nix-table"
```

---
### 6. Selective Termination (proc-1)

- **Status:** ✅ PASSED
- **Conditions:** proc-1 status changes to STOPPED

#### Visual proof
![6. Selective Termination (proc-1)](smoke_step_5.png)

#### Last 10 Console Logs
```
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Processes updated:" "proc-1:running"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
[log] "[NIX] Processes updated:" "proc-1:running, proc-2:running"
[log] "[main] ✅ Snappy scroll settled for #nix-table"
[log] "[NIX] Stop button clicked for:" "proc-1"
[log] "[NIX] Requesting stop for node proc-1"
[log] "[NIX] Stop command acknowledged for proc-1"
[log] "[NIX] Processes updated:" "proc-1:stopped, proc-2:running"
```

---
### 7. Verify proc-2 Persistence

- **Status:** ✅ PASSED
- **Conditions:** proc-2 remains RUNNING

#### Visual proof
![7. Verify proc-2 Persistence](smoke_step_6.png)

#### Last 10 Console Logs
```
[log] "[NIX] Successfully spawned proc-1"
[log] "[NIX] Processes updated:" "proc-1:running"
[log] "[NIX] Spawning new node..."
[log] "[NIX] Successfully spawned proc-2"
[log] "[NIX] Processes updated:" "proc-1:running, proc-2:running"
[log] "[main] ✅ Snappy scroll settled for #nix-table"
[log] "[NIX] Stop button clicked for:" "proc-1"
[log] "[NIX] Requesting stop for node proc-1"
[log] "[NIX] Stop command acknowledged for proc-1"
[log] "[NIX] Processes updated:" "proc-1:stopped, proc-2:running"
```

---