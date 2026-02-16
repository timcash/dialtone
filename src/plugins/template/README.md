# Log 2026-02-16 13:36:14 -0800

- Template domain language aligns with current UI model:
  - each plugin `src_vN` owns a `ui`
  - each `ui` can expose multiple `section`s
  - each `section` uses overlay composition
- Standard overlay contract for new template-based sections:
  - always include `menu`, `thumb`, `legend`
  - include one primary overlay type (`stage`, `table`, `xterm`, or `docs`)
- Section load behavior is dynamic:
  - section overlays load on demand after section selection
  - cached sections should resume without full reload
- `ui_v2` is the shared library for enforcing this language and reducing per-plugin UI duplication.

# Plugin Versioning Workflow (src_vN)

This guide outlines the standard workflow for creating and evolving plugins using versioned source directories (`src_vN`). 

## Quick Start (New Plugin)

Scaffold, install, and verify your new plugin in three commands:

```bash
# 1. Copy the template to your new plugin directory
./dialtone.sh template copy src_v3 src/plugins/my-plugin/src_v1

# 2. Install UI dependencies
./dialtone.sh template install src/plugins/my-plugin/src_v1

# 3. Run the automated test suite
./dialtone.sh go exec run ./src/plugins/my-plugin/src_v1/test
```

---

## Modular UI Sections

The template includes several example sections (Hero, Docs, Table, Three.js, Xterm, Video). **These are modular and optional.** 

- If you don't need a specific feature (e.g., the Video player or Three.js canvas), simply remove its registration in `ui/src/main.ts`, remove the corresponding `<section>` in `ui/index.html`, and delete the component directory in `ui/src/components/`.
- Ensure you also remove the corresponding validation steps in `test/main.go` to keep your tests passing.

---

## Architecture & Conventions

Every versioned plugin implementation lives in its own `src_vN` directory within the plugin's folder (e.g., `src/plugins/my-plugin/src_v1/`).

### Required Structure
A compliant `src_vN` directory MUST contain:
- `cmd/`: Go entrypoint (usually a simple HTTP server for the UI).
- `ui/`: Vite-based TypeScript UI using `@src/libs/ui_v2`.
- `test/`: Go-based test suite using `@src/libs/test_v2`.
- `DESIGN.md`: Architecture and implementation details for this specific version.

### Shared Dependencies
- **UI:** Powered by `@src/libs/ui_v2` for unified styling, section management, and navigation.
- **Tests:** Powered by `@src/libs/test_v2` for automated browser validation, screenshots, and `TEST.md` reporting.

---

## Workflow: Creating a New Plugin

Follow these steps to migrate an existing plugin to the versioned pattern or to start a new version.

### 1. Scaffold the Plugin
Use the `template` plugin to bootstrap your new plugin. This command copies the latest template version, rewrites all package names, paths, and labels, and sets up the basic directory structure.

```bash
# Example: Creating 'my-plugin' version 'src_v1' from template's 'src_v3'
./dialtone.sh template copy src_v3 src/plugins/my-plugin/src_v1
```

### 2. Install Dependencies
Initialize the UI dependencies. This uses `bun` under the hood.

```bash
./dialtone.sh template install src/plugins/my-plugin/src_v1
```

### 3. Verify the Scaffold
Run the automated test suite to ensure the copied template is fully functional in its new location.

```bash
# Run the tests and check the generated report
./dialtone.sh go exec run ./src/plugins/my-plugin/src_v1/test
cat src/plugins/my-plugin/src_v1/test/TEST.md
```

---

## UI Development (`ui_v2`)

The UI is built using a "Section-based" architecture.

### 1. Configuration (`ui/src/main.ts`)
Use `setupApp` to initialize the layout and `sections.register` to define your views.

```typescript
import { setupApp } from '../../../../../libs/ui_v2/ui';

// Initialize the app with a title
const { sections, menu } = setupApp({ title: 'My Plugin', debug: true });

// Register a section
sections.register('overview', {
  containerId: 'overview', // ID of the <section> element in index.html
  load: async () => {
    const { mountOverview } = await import('./components/overview/index');
    return mountOverview(document.getElementById('overview')!);
  },
  header: { title: 'Overview Page' }
});

// Add to the global menu
menu.addButton('Overview', 'Navigate Overview', () => {
  sections.navigateTo('overview');
});
```

### 2. Styling
Import the global theme for consistent Dialtone aesthetics:
```css
/* ui/src/style.css */
@import '../../../../../libs/ui_v2/style.css';
```

---

## Automated Testing (`test_v2`)

Tests are the "Ground Truth" for version readiness.

### 1. Test Structure (`test/main.go`)
Tests are organized into suites of sequential steps.

```go
func main() {
    steps := []test_v2.Step{
        {
            Name: "01 Hero Validation",
            SectionID: "hero",
            Screenshot: "screenshots/hero.png",
            Run: func() error {
                // Use browser actions for validation
                return test_v2.WaitForAriaLabel("Hero Section")
            },
        },
    }

    test_v2.RunSuite(test_v2.SuiteOptions{
        Version:    "src_v1",
        ReportPath: "src/plugins/my-plugin/src_v1/test/TEST.md",
        LogPath:    "src/plugins/my-plugin/src_v1/test/test.log",
    }, steps)
}
```

### 2. Common Browser Actions
- `NavigateToSection(id, ariaLabel)`
- `ClickAriaLabel(label)`
- `TypeAriaLabel(label, value)`
- `AssertAriaLabelTextContains(label, text)`

---

## Integration with `./dialtone.sh`

To make your plugin discoverable via the main CLI, update `src/dev.go` to add your plugin to the main dispatcher:

```go
// src/dev.go
case "my-plugin":
    if err := my_plugin_cli.Run(args); err != nil {
        os.Exit(1)
    }
```

And implement your plugin's CLI dispatcher in `src/plugins/my-plugin/cli/cli.go`, delegating to the `src_vN` directories based on arguments.
