# Nix Plugin

A robust Go-based plugin for managing Nix sub-processes with a modern, section-based Web UI. This plugin provides a 3D visualization of process clusters, detailed documentation, and a fullscreen spreadsheet manager for real-time monitoring.

## CLI Commands

Use the `./dialtone.sh` wrapper to manage the Nix plugin:

```bash
# 🛠️ Development: Start host and UI in dev mode (Vite HMR)
./dialtone.sh nix dev src_v1

# 🏗️ Build: Compile UI assets (TypeScript -> dist/)
./dialtone.sh nix build src_v1

# 💨 Smoke Test: Build + Automated UI verification (Generates SMOKE.md)
./dialtone.sh nix smoke src_v1

# 🧹 Lint: Check Go and TypeScript standards
./dialtone.sh nix lint
```

## Technical Challenges & Solutions

### 1. Component-Based Architecture
**Improvement:** Refactored the UI from a monolithic script into encapsulated TypeScript components (`HeroSection`, `DocsSection`, `TableSection`). Each component manages its own `mount()` and `unmount()` lifecycle.
**Benefit:** Prevents memory leaks and ensures that expensive operations (like 3D rendering or polling) only run when the section is active.

### 2. Robust Smoke Testing with ARIA Labels
**Issue:** `IntersectionObserver` class-based polling (`.is-visible`) was flaky in headless environments due to timing and scroll-snap physics.
**Solution:** Added unique `aria-label` attributes to key section elements (e.g., `aria-label="Nix Process Table"`).
**Fix:** Tests now use `dialtest.NavigateToSection` which waits for these labels. This is 100% deterministic and allows the entire suite to run in seconds.

### 3. Deterministic UI Hiding
**Issue:** Manually toggling styles on global elements (header/menu) from multiple sections led to race conditions.
**Fix:** Implemented global CSS utility classes (`.hide-header`, `.hide-menu`) on the `body`. The `SectionManager` simply toggles these classes based on section config.

### 4. Shared Testing Library
**Improvement:** Extracted common `chromedp` patterns into `src/dialtest/browser.go`.
**Benefit:** Provides a high-level API for other plugins to perform robust SPA navigation and element verification.

### 5. Pulsing Termination Feedback
**Improvement:** Node termination buttons now pulse orange and display "STOPPING..." immediately upon being clicked.
**Benefit:** Provides instant visual feedback to the user, persisting until the backend confirms the process has truly exited.

## What is Nix?

Nix is a tool that takes a unique approach to package management and system configuration. It treats packages like values in a programming language, built in isolation and stored in unique hash-based paths.

---
*Note: This plugin's 'Nix Nodes' currently simulate the management of these isolated environments via host subprocesses.*
