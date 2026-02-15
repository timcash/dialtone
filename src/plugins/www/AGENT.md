# WWW Plugin Progress Report: Architecture & Navigation Overhaul

This document tracks the status of the `www` plugin's navigation and loading lifecycle as of February 15, 2026.

## 🚀 Accomplishments

### 1. Robust Navigation & Spacing
- **Instant Snap**: Eliminated all smooth scrolling and "settle" logic in favor of high-performance CSS scroll-snapping with `scroll-behavior: auto`. This provides instant, reliable transitions between sections.
- **Zero-Dependency Scrolling**: Uninstalled `tinygesture`. Rebuilt navigation using native `wheel`, `touchstart/touchend`, and `keydown` events.
- **Section Isolation**: Added a `25vh` bottom margin to all `.snap-slide` elements. This creates a clear buffer between visualizations and ensures the browser's scroll-snap algorithm reliably targets the intended section without ambiguity.
- **Input Guard**: Implemented a `NAV_COOLDOWN` (400ms) to prevent accidental multi-section jumps during rapid scrolling or swipes.

### 2. Modern Loading Lifecycle
- **Deferred Execution**: Sections now load lazily and start with their Three.js animation loops **paused**. The loop only activates when a section is at least 50% visible.
- **Visual Loading States**:
    - Every section now features a sleek, theme-aware CSS **loading bar**.
    - Main visualization content (`.section-content`) remains at `opacity: 0` until all heavy dependencies (Three.js, land data, etc.) are fully mounted.
- **Ready Signal**: Implemented a standardized `READY: #section-id` console signal. The system transition from "Loading" to "Ready" once the visualization control is fully initialized.

### 3. CI & Smoke Test Enhancements
- **FPS Cleanup**: Removed the fragile FPS-based waiting logic from the smoke test.
- **Signal-Based Verification**: The smoke test (`smoke.go`) now dynamically listens for the `READY:` log before capturing screenshots, significantly increasing reliability.
- **Speed Optimization**: Reduced redundant `time.Sleep` calls and shortened timeouts from 30s to 8s per section.

---

## 🛠 Current Status
- **Performance**: High. GPU usage is minimized by pausing inactive sections.
- **Reliability**: Excellent on Desktop. Mobile touch navigation is implemented and snappy.
- **Verification**: `smoke.go` is functional and verifying all 11 sections correctly.

---

## 📋 Next Steps

### 1. Code Splitting & Three.js Optimization
- **Goal**: Further delay the loading of the core `three` library until the first 3D section is actually scrolled into view.
- **Task**: Audit `package.json` and Vite chunks to ensure `three` isn't in the initial `main` bundle.

### 2. Smoke Test Parallelization
- **Goal**: Get the smoke test duration under 30 seconds total.
- **Task**: Explore running non-UI performance checks in parallel while serializing the screenshot captures.

### 3. Mobile Stability Audit
- **Task**: Confirm that the new `touchend` logic completely resolves the intermittent "stuck" state previously seen on the policy section.
- **Task**: Check if the `25vh` margin needs to be scaled for very small mobile screens.

---

## 💡 Instructions for the Next Agent
1. **Verify Baseline**: Run `export SMOKE_HEADLESS=true && ./dialtone.sh www smoke` to ensure the current snapping/loading logic passes.
2. **Component Cleanup**: Ensure any new sections follow the pattern in `src/components/util/section.ts` and emit the `READY:` signal.
3. **Debug Logs**: Terminal logs will show `[APP] [SectionManager] READY: #s-id` when a section finishes loading. If this doesn't appear, the visualization's `mount` function might be hanging.
