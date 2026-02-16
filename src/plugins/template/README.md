# Plugin Versioning Workflow (src_vN)

This guide outlines the standard workflow for creating and evolving plugins using versioned source directories (`src_vN`). This pattern ensures stability, enables safe experimentation, and provides a clear migration path between major implementations.

## Architecture & Conventions

Every versioned plugin implementation lives in its own `src_vN` directory within the plugin's folder (e.g., `src/plugins/my-plugin/src_v1/`).

### Required Structure
A compliant `src_vN` directory MUST contain:
- `cmd/`: Go entrypoint (usually a simple HTTP server for the UI).
- `ui/`: Vite-based TypeScript UI.
- `test/`: `test_v2` suite providing automated browser and logic validation.
- `DESIGN.md`: Architecture and implementation details for this specific version.

### Shared Dependencies
Versioned UIs and Tests leverage shared libraries to maintain consistency:
- **UI:** `@import '../../../../../libs/ui_v2/style.css'` for unified styling.
- **Tests:** `dialtone/cli/src/libs/test_v2` for automated reporting and screenshots.

---

## Workflow: Creating a New Plugin Version

Follow these steps to migrate an existing plugin to the versioned pattern or to start a new version.

### 1. Scaffold the Version
Use the `template` plugin's latest version as a base.

```bash
# Example: Creating v1 for a new 'my-plugin'
mkdir -p src/plugins/my-plugin/src_v1
cp -r src/plugins/template/src_v3/* src/plugins/my-plugin/src_v1/
```

### 2. Customize the Implementation
Update all references to `template` and `src_v3` in the copied files:
- **Go Server (`cmd/main.go`):** Update the fallback path to your plugin directory.
- **UI (`ui/package.json`):** Update the package name and description.
- **UI (`ui/src/main.ts`):** Update the `setupApp` title.
- **Tests (`test/main.go`):** Update the `Version` and artifact paths.

### 3. Implement the Plugin Logic
Develop your feature within the `src_vN` scope. 
- Avoid animations or fades in the UI to keep tests fast and deterministic.
- Use `aria-label` extensively for all interactive elements to support stable testing.

### 4. Hook up the CLI
Update your plugin's CLI dispatcher (e.g., `src/plugins/my-plugin/cli/my-plugin.go`) to support versioned subcommands:

```go
func RunMyPlugin(args []string) {
    // ... dispatcher logic ...
    switch subcommand {
    case "install": RunInstall(getDir())
    case "test":    RunTest(getDir())
    case "build":   RunBuild(getDir())
    // ...
    }
}
```

---

## Testing & Validation Workflow

Testing is mandatory for all `src_vN` implementations and is the "Ground Truth" for version readiness.

### 1. Daily Development
Run the versioned dev server with browser synchronization:
```bash
./dialtone.sh my-plugin dev src_v1
```

### 2. Continuous Testing
Run the full automated suite frequently during development:
```bash
./dialtone.sh my-plugin test src_v1
```

### 3. Review Artifacts
Every `test` run produces a `TEST.md` report. **Always review this file before committing.**
```bash
# Check step status and embedded screenshots
cat src/plugins/my-plugin/src_v1/test/TEST.md
```

---

## Migration Lessons (from Cloudflare Plugin)

- **Immediate Swaps:** Do not use CSS transitions for section changes. Tests expect elements to be interactable immediately.
- **Lifecycle Logs:** Ensure `SectionManager` lifecycle events are logged. Tests verify these to ensure components aren't leaking memory or state.
- **Cleanup is Critical:** Always implement thorough cleanup in `test/18.go` (or your final cleanup step) to ensure ports are released for subsequent runs.
- **ARIA First:** If an element is hard to select in a test, add an `aria-label` instead of using complex CSS selectors.
