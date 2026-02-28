# Error Report

- **Date**: Sat, 28 Feb 2026 12:43:26 PST
- **Suite**: src-v1-self-check
- **Total Duration**: 5m34.547522305s

- **Error Steps**: 3 / 4

## 1. ctx-logging-and-waits

- **Duration**: 4.419079ms

### Step Errors

```text
ERROR: ctx error message
ERROR: ctx error message
ERROR: ctx error format check
```

---

## 3. example-template-step

- **Duration**: 1.84966ms

### Step Errors

```text
ERROR: template plugin error
```

---

## 4. example-browser-stepcontext-api

- **Duration**: 5m31.213793177s
- **Step Error**: `step example-browser-stepcontext-api timed out`

### Step Errors

```text
FAIL: [TEST][FAIL] [STEP:example-browser-stepcontext-api] timed out after 20s
```

---

