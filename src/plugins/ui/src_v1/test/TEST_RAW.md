# Test Report: ui-src-v1

- **Date**: Fri, 27 Feb 2026 18:58:13 PST
- **Total Duration**: 6.286751593s

## Summary

- **Steps**: 10 / 10 passed
- **Status**: PASSED

## Details

### 1. ✅ ui-quality-fmt-lint-build

- **Duration**: 2.220064772s
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
INFO: stdout: dist/assets/index-DU0jfcrJ.css   13.13 kB │ gzip:   3.51 kB
INFO: stdout: dist/assets/index-ZGf0pex0.js   510.71 kB │ gzip: 130.26 kB
INFO: stdout: ✓ built in 698ms
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

- **Duration**: 4.056446124s
- **Report**: fixture served and attached browser session is ready

#### Logs

```text
INFO: STEP> begin ui-build-and-go-serve
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
WARN: INJECTED_BROWSER_CHECK: skipped (remote browser mode)
INFO: saved browser debug config: /home/user/dialtone/src/plugins/ui/src_v1/test/browser.debug.json
INFO: report: fixture served and attached browser session is ready
PASS: [TEST][PASS] [STEP:ui-build-and-go-serve] report: fixture served and attached browser session is ready
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_hero.png](screenshots/ui_hero.png)

---

### 3. ✅ ui-section-hero-via-menu

- **Duration**: 2.192959ms
- **Report**: section hero attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-hero-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section hero attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-hero-via-menu] report: section hero attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_hero_section.png](screenshots/ui_hero_section.png)

---

### 4. ✅ ui-section-three-fullscreen-via-menu

- **Duration**: 1.137325ms
- **Report**: section three-fullscreen attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-three-fullscreen-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section three-fullscreen attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-three-fullscreen-via-menu] report: section three-fullscreen attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_three_fullscreen.png](screenshots/ui_three_fullscreen.png)

---

### 5. ✅ ui-section-three-calculator-via-menu

- **Duration**: 960.439µs
- **Report**: section three-calculator attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-three-calculator-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section three-calculator attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-three-calculator-via-menu] report: section three-calculator attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_three_calculator_section.png](screenshots/ui_three_calculator_section.png)

---

### 6. ✅ ui-section-table-via-menu

- **Duration**: 1.39504ms
- **Report**: section table attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-table-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section table attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-table-via-menu] report: section table attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_table.png](screenshots/ui_table.png)

---

### 7. ✅ ui-section-camera-via-menu

- **Duration**: 1.41477ms
- **Report**: section camera attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-camera-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section camera attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-camera-via-menu] report: section camera attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_camera.png](screenshots/ui_camera.png)

---

### 8. ✅ ui-section-docs-via-menu

- **Duration**: 991.774µs
- **Report**: section docs attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-docs-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section docs attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-docs-via-menu] report: section docs attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_docs.png](screenshots/ui_docs.png)

---

### 9. ✅ ui-section-terminal-via-menu

- **Duration**: 812.933µs
- **Report**: section terminal attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-terminal-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section terminal attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-terminal-via-menu] report: section terminal attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_terminal_section.png](screenshots/ui_terminal_section.png)

---

### 10. ✅ ui-section-settings-via-menu

- **Duration**: 1.30496ms
- **Report**: section settings attach setup verified

#### Logs

```text
INFO: STEP> begin ui-section-settings-via-menu
INFO: LOOKING FOR: persistent ui dev server already running at http://127.0.0.1:5177
INFO: report: section settings attach setup verified
PASS: [TEST][PASS] [STEP:ui-section-settings-via-menu] report: section settings attach setup verified
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![ui_settings.png](screenshots/ui_settings.png)

---

