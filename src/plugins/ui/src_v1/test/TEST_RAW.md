# Test Report: ui-src-v1

- **Date**: Fri, 27 Feb 2026 14:56:02 PST
- **Total Duration**: 16.689177864s

## Summary

- **Steps**: 10 / 10 passed
- **Status**: PASSED

## Details

### 1. ✅ ui-quality-fmt-lint-build

- **Duration**: 2.435028291s
- **Report**: fmt-check, lint, and build passed

#### Logs

```text
INFO: running command: /home/user/dialtone/dialtone.sh ui src_v1 install
INFO: stdout: >> Running: /home/user/dialtone_dependencies/bun/bin/bun install (in /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app)
INFO: stdout: bun install v1.3.9 (cf6cdbbb)
INFO: stdout: Checked 22 installs across 69 packages (no changes) [1.00ms]
INFO: stderr: <empty>
INFO: running command: /home/user/dialtone/dialtone.sh ui src_v1 fmt-check
INFO: stdout: >> Running: /home/user/dialtone_dependencies/bun/bin/bun run fmt:check (in /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app)
INFO: stdout: Checking formatting...
INFO: stdout: All matched files use Prettier code style!
INFO: stderr: $ prettier --check .
INFO: running command: /home/user/dialtone/dialtone.sh ui src_v1 lint
INFO: stdout: >> Running: /home/user/dialtone_dependencies/bun/bin/bun run lint (in /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app)
INFO: stderr: $ tsc --noEmit
INFO: running command: /home/user/dialtone/dialtone.sh ui src_v1 build
INFO: stdout: >> Running: /home/user/dialtone_dependencies/bun/bin/bun run build (in /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app)
INFO: stdout: vite v5.4.21 building for production...
INFO: stdout: transforming...
INFO: stdout: ✓ 12 modules transformed.
INFO: stdout: rendering chunks...
INFO: stdout: computing gzip size...
INFO: stdout: dist/index.html                   1.56 kB │ gzip:   0.47 kB
INFO: stdout: dist/assets/index-DajVPu_L.css   13.19 kB │ gzip:   3.53 kB
INFO: stdout: dist/assets/index-BXDP4L3t.js   511.08 kB │ gzip: 130.40 kB
INFO: stdout: ✓ built in 761ms
INFO: stderr: $ vite build
INFO: stderr: (!) Some chunks are larger than 500 kB after minification. Consider:
INFO: stderr: - Using dynamic import() to code-split the application
INFO: stderr: - Use build.rollupOptions.output.manualChunks to improve chunking: https://rollupjs.org/configuration-options/#output-manualchunks
INFO: stderr: - Adjust chunk size limit for this warning via build.chunkSizeWarningLimit.
INFO: report: fmt-check, lint, and build passed
PASS: [TEST][PASS] [STEP:ui-quality-fmt-lint-build] report: fmt-check, lint, and build passed
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ ui-build-and-go-serve

- **Duration**: 5.390980978s
- **Report**: fixture built, hero section loaded, legend header verified (attach=true)

#### Logs

```text
INFO: STEP> begin ui-build-and-go-serve
INFO: LOOKING FOR: ui fixture build at /home/user/dialtone/src/plugins/ui/src_v1/test/fixtures/app
INFO: LOOKING FOR: [/home/user/dialtone_dependencies/bun/bin/bun install --silent]
INFO: LOOKING FOR: [/home/user/dialtone_dependencies/bun/bin/bun run build]
INFO: LOOKING FOR: go ui backend at http://127.0.0.1:38589
INFO: INJECTED_BROWSER_CHECK: start browser_subject=logs.test.ui.src-v1.ui-build-and-go-serve.browser error_subject=logs.test.ui.src-v1.error
INFO: INJECTED_BROWSER_CHECK: browser-topic-ok marker=__DIALTONE_INJECTED_BROWSER_TOPIC__:1772232952991013187
INFO: INJECTED_BROWSER_CHECK: error-topic-ok marker=__DIALTONE_INJECTED_BROWSER_TOPIC__:1772232952991013187:error
INFO: INJECTED_BROWSER_CHECK: pass browser_topic=true error_topic=true
INFO: report: fixture built, hero section loaded, legend header verified (attach=true)
PASS: [TEST][PASS] [STEP:ui-build-and-go-serve] report: fixture built, hero section loaded, legend header verified (attach=true)
```

#### Browser Logs

```text
INFO: CONSOLE:log: "__DIALTONE_INJECTED_BROWSER_TOPIC__:1772232952991013187"
ERROR: ERROR: Uncaught Error: __DIALTONE_INJECTED_BROWSER_TOPIC__:1772232952991013187:error
    at <anonymous>:3:28
```

#### Screenshots

![ui_hero.png](screenshots/ui_hero.png)

---

### 3. ✅ ui-section-docs-via-menu

- **Duration**: 814.608891ms
- **Report**: section docs navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-docs-via-menu
INFO: OVERLAP: section=docs check=start
INFO: OVERLAP: section=docs none
INFO: report: section docs navigation verified
PASS: [TEST][PASS] [STEP:ui-section-docs-via-menu] report: section docs navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Docs"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #docs"
INFO: CONSOLE:log: "[SectionManager] LOADING #docs"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #docs"
INFO: CONSOLE:log: "[SectionManager] LOADED #docs"
INFO: CONSOLE:log: "[SectionManager] START #docs"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #docs"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #docs"
INFO: CONSOLE:log: "[SectionManager] RESUME #docs"
```

#### Screenshots

![ui_docs.png](screenshots/ui_docs.png)

---

### 4. ✅ ui-section-table-via-menu

- **Duration**: 785.867326ms
- **Report**: section table navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-table-via-menu
INFO: OVERLAP: section=table check=start
INFO: OVERLAP: section=table none
INFO: report: section table navigation verified
PASS: [TEST][PASS] [STEP:ui-section-table-via-menu] report: section table navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #docs"
INFO: CONSOLE:log: "[SectionManager] PAUSE #docs"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Table"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #table"
INFO: CONSOLE:log: "[SectionManager] LOADING #table"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #table"
INFO: CONSOLE:log: "[SectionManager] LOADED #table"
INFO: CONSOLE:log: "[SectionManager] START #table"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #table"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #table"
INFO: CONSOLE:log: "[SectionManager] RESUME #table"
```

#### Screenshots

![ui_table.png](screenshots/ui_table.png)

---

### 5. ✅ ui-section-three-fullscreen-via-menu

- **Duration**: 816.734532ms
- **Report**: section three-fullscreen navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-three-fullscreen-via-menu
INFO: OVERLAP: section=three-fullscreen check=start
INFO: OVERLAP: section=three-fullscreen none
INFO: report: section three-fullscreen navigation verified
PASS: [TEST][PASS] [STEP:ui-section-three-fullscreen-via-menu] report: section three-fullscreen navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #table"
INFO: CONSOLE:log: "[SectionManager] PAUSE #table"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Three Fullscreen"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] LOADING #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] LOADED #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] START #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] RESUME #three-fullscreen"
```

#### Screenshots

![ui_three_fullscreen.png](screenshots/ui_three_fullscreen.png)

---

### 6. ✅ ui-section-camera-via-menu

- **Duration**: 995.844103ms
- **Report**: section camera navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-camera-via-menu
INFO: OVERLAP: section=camera check=start
INFO: OVERLAP: section=camera none
INFO: report: section camera navigation verified
PASS: [TEST][PASS] [STEP:ui-section-camera-via-menu] report: section camera navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] PAUSE #three-fullscreen"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Camera"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #camera"
INFO: CONSOLE:log: "[SectionManager] LOADING #camera"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #camera"
INFO: CONSOLE:log: "[SectionManager] LOADED #camera"
INFO: CONSOLE:log: "[SectionManager] START #camera"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #camera"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #camera"
INFO: CONSOLE:log: "[SectionManager] RESUME #camera"
```

#### Screenshots

![ui_camera.png](screenshots/ui_camera.png)

---

### 7. ✅ ui-section-settings-via-menu

- **Duration**: 710.748755ms
- **Report**: section settings navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-settings-via-menu
INFO: OVERLAP: section=settings check=start
INFO: OVERLAP: section=settings overlay:menu/-(-) <-> button:-/-(-) area=2268.0px a=55.37%!b(MISSING)=15.65%!a(MISSING)llowedByMenu=true
INFO: OVERLAP: section=settings overlay:menu/-(-) <-> button:-/-(-) area=108.0px a=2.64%!b(MISSING)=0.75%!a(MISSING)llowedByMenu=true
INFO: report: section settings navigation verified
PASS: [TEST][PASS] [STEP:ui-section-settings-via-menu] report: section settings navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #camera"
INFO: CONSOLE:log: "[SectionManager] PAUSE #camera"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Settings"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #settings"
INFO: CONSOLE:log: "[SectionManager] LOADING #settings"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #settings"
INFO: CONSOLE:log: "[SectionManager] LOADED #settings"
INFO: CONSOLE:log: "[SectionManager] START #settings"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #settings"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #settings"
INFO: CONSOLE:log: "[SectionManager] RESUME #settings"
```

#### Screenshots

![ui_settings.png](screenshots/ui_settings.png)

---

### 8. ✅ ui-section-terminal-via-menu

- **Duration**: 846.934299ms
- **Report**: section terminal navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-terminal-via-menu
INFO: OVERLAP: section=terminal check=start
INFO: OVERLAP: section=terminal none
INFO: report: section terminal navigation verified
PASS: [TEST][PASS] [STEP:ui-section-terminal-via-menu] report: section terminal navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #settings"
INFO: CONSOLE:log: "[SectionManager] PAUSE #settings"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #terminal"
INFO: CONSOLE:log: "[SectionManager] LOADING #terminal"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #terminal"
INFO: CONSOLE:log: "[SectionManager] LOADED #terminal"
INFO: CONSOLE:log: "[SectionManager] START #terminal"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #terminal"
INFO: CONSOLE:log: "[SectionManager] RESUME #terminal"
```

#### Screenshots

![ui_terminal_section.png](screenshots/ui_terminal_section.png)

---

### 9. ✅ ui-section-three-calculator-via-menu

- **Duration**: 903.622891ms
- **Report**: section three-calculator navigation verified

#### Logs

```text
INFO: STEP> begin ui-section-three-calculator-via-menu
INFO: OVERLAP: section=three-calculator check=start
INFO: OVERLAP: section=three-calculator none
INFO: report: section three-calculator navigation verified
PASS: [TEST][PASS] [STEP:ui-section-three-calculator-via-menu] report: section three-calculator navigation verified
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #terminal"
INFO: CONSOLE:log: "[SectionManager] PAUSE #terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #hero"
INFO: CONSOLE:log: "[SectionManager] RESUME #hero"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Three Calculator"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #three-calculator"
INFO: CONSOLE:log: "[SectionManager] LOADING #three-calculator"
INFO: CONSOLE:log: "[SectionManager] ctl.load() RESOLVED for #three-calculator"
INFO: CONSOLE:log: "[SectionManager] LOADED #three-calculator"
INFO: CONSOLE:log: "[SectionManager] START #three-calculator"
INFO: CONSOLE:log: "[SectionManager] Setting data-ready=true on #three-calculator"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #hero"
INFO: CONSOLE:log: "[SectionManager] PAUSE #hero"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #three-calculator"
INFO: CONSOLE:log: "[SectionManager] RESUME #three-calculator"
```

#### Screenshots

![ui_three_calculator_section.png](screenshots/ui_three_calculator_section.png)

---

### 10. ✅ ui-component-actions-and-modes

- **Duration**: 2.874139644s
- **Report**: component actions verified (mode toggle, table refresh, terminal send, three add) with mobile screenshots

#### Logs

```text
INFO: STEP> begin ui-component-actions-and-modes
INFO: report: component actions verified (mode toggle, table refresh, terminal send, three add) with mobile screenshots
PASS: [TEST][PASS] [STEP:ui-component-actions-and-modes] report: component actions verified (mode toggle, table refresh, terminal send, three add) with mobile screenshots
```

#### Browser Logs

```text
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #table"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #three-calculator"
INFO: CONSOLE:log: "[SectionManager] PAUSE #three-calculator"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #table"
INFO: CONSOLE:log: "[SectionManager] RESUME #table"
INFO: CONSOLE:log: "table-refreshed"
INFO: CONSOLE:log: "mode-toggle:table:fullscreen"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #table"
INFO: CONSOLE:log: "[SectionManager] PAUSE #table"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #terminal"
INFO: CONSOLE:log: "[SectionManager] RESUME #terminal"
INFO: CONSOLE:log: "log-submit:ok"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Toggle Global Menu"
INFO: CONSOLE:log: "[TEST_ACTION] click aria=Navigate Three Calculator"
INFO: CONSOLE:log: "[SectionManager] NAVIGATING TO #three-calculator"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE AWAY #terminal"
INFO: CONSOLE:log: "[SectionManager] PAUSE #terminal"
INFO: CONSOLE:log: "[SectionManager] NAVIGATE TO #three-calculator"
INFO: CONSOLE:log: "three-add:1"
INFO: CONSOLE:log: "[SectionManager] RESUME #three-calculator"
```

#### Screenshots

![ui_table_fullscreen.png](screenshots/ui_table_fullscreen.png)
![ui_terminal.png](screenshots/ui_terminal.png)
![ui_three_calculator.png](screenshots/ui_three_calculator.png)

---

