# Test Report: cad-src-v1

- **Date**: Thu, 19 Mar 2026 15:49:05 PDT
- **Total Duration**: 27.815270754s

## Summary

- **Steps**: 1 / 1 passed
- **Status**: PASSED

## Details

### 1. ✅ cad-ui-browser-smoke-src-v1

- **Duration**: 27.099309989s
- **Report**: CAD UI loaded in chrome src_v3 with no browser exceptions and STL model reached ready state

#### Logs

```text
INFO: cad browser smoke server ready at http://127.0.0.1:42935
WARN: ERROR_PING: skipped for chrome src_v3 NATS-managed browser session
INFO: report: CAD UI loaded in chrome src_v3 with no browser exceptions and STL model reached ready state
PASS: [TEST][PASS] [STEP:cad-ui-browser-smoke-src-v1] report: CAD UI loaded in chrome src_v3 with no browser exceptions and STL model reached ready state
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] LOADING #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] LOADED #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] START #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #cad-three-stage"
INFO: CONSOLE:log: "[SectionManager] RESUME #cad-three-stage"
INFO: CONSOLE:log: "cad-model-ready:1"
INFO: CONSOLE:log: "cad-model-ready:2"
INFO: CONSOLE:log: "cad-model-ready:3"
INFO: CONSOLE:log: "cad-model-ready:4"
INFO: CONSOLE:log: "cad-model-ready:5"
```

#### Screenshots

![auto_cad-ui-browser-smoke-src-v1.png](screenshots/auto_cad-ui-browser-smoke-src-v1.png)
![cad_ui_browser_smoke.png](screenshots/cad_ui_browser_smoke.png)

---

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `13`

| PID | ROLE | PORT |
| --- | --- | --- |
| 1148 | `unlabeled` | 21944 |
| 2996 | `cad-smoke` | 21944 |
| 3084 | `unlabeled` | 22602 |
| 4296 | `dev-isolated` | 22602 |
| 5956 | `unlabeled` | 21944 |
| 10076 | `unlabeled` | 21944 |
| 11952 | `unlabeled` | 22602 |
| 14156 | `dev` | 19464 |
| 14500 | `unlabeled` | 19464 |
| 17200 | `unlabeled` | 21944 |
| 17492 | `unlabeled` | 19464 |
| 19132 | `unlabeled` | 19464 |
| 19640 | `unlabeled` | 22602 |

<!-- DIALTONE_CHROME_REPORT_END -->
