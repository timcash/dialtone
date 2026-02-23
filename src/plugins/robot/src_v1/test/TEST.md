# Test Report: src_v1

- **Date**: Mon, 23 Feb 2026 11:15:01 PST
- **Total Duration**: 25.864288742s

## Summary

- **Steps**: 15 / 15 passed
- **Status**: PASSED

## Details

### 1. ✅ 00 Reset Workspace

- **Duration**: 740.477656ms
- **Report**: Reset workspace: cleaned ports 3000/test-web.

---

### 2. ✅ 01 Preflight (Go/UI)

- **Duration**: 9.020490396s
- **Report**: Preflight checks passed.

---

### 3. ✅ 02 Go Run (Mock Server Check)

- **Duration**: 853.152418ms
- **Report**: Go server running.

---

### 4. ✅ 03 UI Run

- **Duration**: 1.010256006s
- **Report**: UI server check passed.

---

### 5. ✅ 04 Expected Errors (Proof of Life)

- **Duration**: 5.324424292s
- **Report**: Proof of life errors detected.

---

### 6. ✅ 05 Dev Server Running (latest UI)

- **Duration**: 1.03556049s
- **Report**: Dev server serves latest UI.

---

### 7. ✅ 06 Hero Section Validation

- **Duration**: 167.605013ms
- **Report**: Hero section and canvas validated visible.

#### Screenshots

![test_step_1.png](screenshots/test_step_1.png)

---

### 8. ✅ 07 Docs Section Validation

- **Duration**: 564.280098ms
- **Report**: Docs section navigation and content validated.

#### Screenshots

![test_step_2.png](screenshots/test_step_2.png)

---

### 9. ✅ 08 Table Section Validation

- **Duration**: 1.047560837s
- **Report**: Table section validated with data rows.

#### Screenshots

![test_step_3.png](screenshots/test_step_3.png)

---

### 10. ✅ 09 Three Section Validation

- **Duration**: 537.71432ms
- **Report**: Three section navigation and canvas validated.

#### Screenshots

![test_step_4.png](screenshots/test_step_4.png)

---

### 11. ✅ 10 Xterm Section Validation

- **Duration**: 872.837994ms
- **Report**: Xterm section validated with command execution.

#### Screenshots

![test_step_5.png](screenshots/test_step_5.png)

---

### 12. ✅ 11 Video Section Validation

- **Duration**: 929.24765ms
- **Report**: Video section validated with playback.

#### Screenshots

![test_step_6.png](screenshots/test_step_6.png)

---

### 13. ✅ 12 Lifecycle / Invariants

- **Duration**: 3.041848789s
- **Report**: Lifecycle invariants maintained.

---

### 14. ✅ 13 Menu Navigation Validation

- **Duration**: 708.756318ms
- **Report**: Menu navigation flow validated.

#### Screenshots

![menu_1_hero.png](screenshots/menu_1_hero.png)
![menu_2_open.png](screenshots/menu_2_open.png)
![menu_3_telemetry.png](screenshots/menu_3_telemetry.png)

---

### 15. ✅ 14 Cleanup Verification

- **Duration**: 5.005276ms
- **Report**: Cleanup successful (teardown called, web port=43810).

---

