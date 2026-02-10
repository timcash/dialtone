# Template Plugin

A minimal, stable reference implementation for creating new plugins with versioned UI components and robust smoke testing.

## CLI Commands

```bash
# 🏗️ Build: Compile UI assets
./dialtone.sh template build src_v1

# 💨 Smoke Test: Automated UI verification
./dialtone.sh template smoke src_v1

# 🆕 Versioning: Scaffold next src_vN folder
./dialtone.sh template new-version
```

## Key Features

### 1. Component Architecture
Each section is a standalone TypeScript class implementing the `SectionComponent` interface.
- `mount()`: Called when the section is first loaded.
- `unmount()`: Called for cleanup.
- `setVisible()`: Handles visibility transitions.

### 2. Deterministic UI State
Global UI elements (like the header and menu) are hidden via simple CSS utility classes on the `body`.
- `body.hide-header`: Hides the main header.
- `body.hide-menu`: Hides the navigation menu.

Section configurations in `main.ts` control these classes:
```typescript
sections.register('settings', { 
  component: SettingsSection, 
  header: { visible: false, menuVisible: false } 
});
```

### 3. Robust Testing
Uses the `dialtest` library (`src/dialtest/browser.go`) for high-speed, reliable browser automation.
- **ARIA Labels:** Key elements use `aria-label` for 100% reliable detection.
- **Unified Navigation:** `dialtest.NavigateToSection` ensures the UI state is fully updated before proceeding.
- **Visual Proof:** Automated screenshots are captured for every step and rendered in `SMOKE.md`.

## Scaffolding New Versions
Use `./dialtone.sh template new-version` to clone the latest `src_vN` directory and increment the version number. This allows for safe, incremental development of new features.