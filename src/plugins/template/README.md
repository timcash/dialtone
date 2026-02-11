# Template Plugin

The **Template Plugin** is a robust, stable reference implementation for building Dialtone plugins. It demonstrates versioned UI development, lazy-loading architecture, and comprehensive automated smoke testing.

## System Architecture

### 1. Versioned Source Folders (`src_vN`)
The plugin follows an incremental versioning pattern (`src_v1`, `src_v2`, etc.). This allows developers to:
- Experiment with new UI patterns without breaking existing stable versions.
- Maintain legacy code while transitioning to new architectures.
- Run side-by-side comparisons and smoke tests across versions.

### 2. UI Library & Patterns (`src/libs/ui`)
The plugin utilizes a shared UI library located in `src/libs/ui`. This library provides core utilities:
- **`SectionManager`**: Orchestrates the SPA (Single Page Application) workflow. It handles lazy-loading components, updating the global header, and managing the visibility of sections.
- **`Menu`**: A standard side-navigation system.
- **`VisibilityMixin`**: Standardizes lifecycle logging (`SLEEP`, `AWAKE`).
- **Styles**: Consolidated CSS variables and layouts in `src/libs/ui/style.css`.

### 3. Lifecycle Management
Sections in the template plugin go through a managed lifecycle, which is logged to the console for both debugging and automated verification:
- **📦 LOADING**: The browser starts fetching the JavaScript chunk for the section.
- **✅ LOADED**: The code is loaded and the component is mounted.
- **✨ START**: The component initialization logic runs.
- **🚀 RESUME / AWAKE**: The section becomes visible; animation loops should start.
- **💤 PAUSE / SLEEP**: The section is hidden; animation loops should suspend to save resources.
- **🧭 NAVIGATING TO**: Explicit log indicating a user or test-initiated transition.

---

## Smoke Testing Framework

The Template plugin features a high-fidelity automation suite powered by the `dialtest` library.

### `SmokeRunner` (Centralized Engine)
The `SmokeRunner` in `src/libs/dialtest/smoke.go` abstracts the complexity of browser automation:
- **Automatic Browser Management**: Launches a dedicated, headless Chrome instance and ensures it is cleaned up via the Chrome Plugin API.
- **Unified Logging**: Redirects browser console logs and Go-side logs into a single `smoke.log` file.
- **Preflight Checks**: Automatically runs `install`, `lint`, and `build` before starting the UI tests.
- **Server Orchestration**: Manages the plugin's Go server lifecycle during the test run.

### Test Artifacts (The `smoke/` folder)
Every version (`src_vN`) has a `smoke/` directory containing:
- **`smoke.go`**: The test definition (steps, assertions, and navigation).
- **`SMOKE.md`**: A detailed markdown report generated after every run.
- **`smoke.log`**: A complete audit trail of the test execution.
- **`smoke_step_N.png`**: Screenshots captured at every validation step.

### `SMOKE.md` Structure
The generated report provides absolute proof of system health:
1.  **Expected Errors (Proof of Life)**: Deliberate errors triggered to verify the logging pipeline is actually working.
2.  **Real Errors & Warnings**: Any unexpected console issues or exceptions found during navigation.
3.  **Preflight Results**: Logs from the build and lint steps.
4.  **UI & Interactivity**: For each step, it shows the **intent**, the **browser logs**, and a **screenshot**.
5.  **Lifecycle Verification Summary**: A table confirming that every section correctly performed its Load/Start/Pause/Resume transitions.

---

## CLI Commands

### 🏗️ Build
Compiles the UI assets for a specific version.
```bash
./dialtone.sh template build src_v2
```

### 💨 Smoke Test
Runs the full automated suite for a specific version.
```bash
./dialtone.sh template smoke src_v2
```

### 🆕 Scaffold New Version
Clones the latest version to a new number, providing a fresh starting point.
```bash
# Creates src_v3 from the latest src_vN
./dialtone.sh template src --n 3
```

### 🛠️ Development Mode
Runs the Vite dev server for rapid UI iteration.
```bash
./dialtone.sh template dev src_v2
```

## Creating a New Section
To add a new section to the template:
1.  Create a component folder in `src_vN/ui/src/components/`.
2.  Implement the `mount` function and use `VisibilityMixin`.
3.  Register the section in `main.ts`:
    ```typescript
    sections.register('my-section', {
        containerId: 'my-container',
        load: async () => {
            const { mountMySection } = await import('./components/my-section/index');
            const container = document.getElementById('my-container');
            return mountMySection(container!);
        },
        header: { visible: true, title: 'My New Section' }
    });
    ```
4.  Add a button to the `menu` in `main.ts`.
5.  Add a validation step in `smoke/smoke.go`.
