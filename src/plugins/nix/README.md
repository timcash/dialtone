# Nix Plugin

A robust Go-based plugin for managing Nix sub-processes with a modern, section-based Web UI. This plugin provides a 3D visualization of process clusters, detailed documentation, and a fullscreen spreadsheet manager for real-time monitoring.

## CLI Commands

Use the `./dialtone.sh` wrapper to manage the Nix plugin:

```bash
# Start the Nix host and UI in development mode
./dialtone.sh nix dev src_v1

# Build the UI assets (Vite + TypeScript)
./dialtone.sh nix build src_v1

# Run high-speed automated UI tests (generates SMOKE.md)
# This command automatically runs a 'build' before starting.
./dialtone.sh nix smoke src_v1

# Lint Go and TypeScript code
./dialtone.sh nix lint
```

## Technical Challenges & Solutions

During development, we encountered several architectural hurdles that required specific fixes for reliability and UX:

### 1. Reliable Chromedp Navigation
**Issue:** Standard hash-based navigation (`#nix-table`) was unreliable for automated screenshots. The browser would often capture the "in-between" state during scroll animations.
**Fix:** Refactored the UI to use a custom `SectionManager` router. It now exposes a `window.navigateTo` function that emits a `section-nav-complete` event. The smoke test waits for this event rather than using static sleeps, making tests faster and 100% accurate.

### 2. Fullscreen Table Layout
**Issue:** The Nix process table originally had surrounding grey space and padding, making it look like a "widget" rather than a professional tool.
**Fix:** Refactored the snapping logic from a `main` container to the `body` level. Removed all padding and borders from the `.explorer-container`, resulting in a true edge-to-edge `100vw/100vh` data view.

### 3. Context-Aware Menu Visibility
**Issue:** The global menu button overlapped with the fullscreen table and distracted from the data-heavy view.
**Fix:** Updated the `Menu` class with a `setVisible(bool)` API. The `SectionManager` now automatically hides the menu button when entering the `nix-table` section and restores it when leaving.

### 4. Process Status Synchronization
**Issue:** Killing a process via the API would sometimes show it as "running" for several seconds because the OS hadn't updated the `ProcessState` yet.
**Fix:** Implemented an internal status map in the Go backend. We now explicitly set the state to `stopped` immediately upon successful `Kill()`, ensuring the UI reflects the change instantly.

## What is Nix?

Nix is a tool that takes a unique approach to package management and system configuration. 

### How it works
Nix is "functional"—it treats packages like values in a programming language. They are built in isolation and stored in unique paths in the `/nix/store` (e.g., `/nix/store/b697...-firefox-1.0`). This allows multiple versions of the same package to coexist without conflict.

### Platform Support
*   **macOS:** Nix uses a multi-user installation that creates a dedicated encrypted APFS volume for `/nix`. It integrates seamlessly with Zsh and Bash on Apple Silicon and Intel Macs.
*   **Linux:** The primary home of Nix. It works on any distribution and is the foundation of NixOS, where the entire OS is defined declaratively.
*   **Windows:** Nix runs via **WSL2** (Windows Subsystem for Linux). Users typically install Ubuntu or Alpine in WSL and then install Nix inside that environment to manage their Windows development tools.

---
*Note: This plugin's 'Nix Nodes' currently simulate the management of these isolated environments via host subprocesses.*