# Handoff: Smoke Test Overhaul
> [!NOTE]
> **How to Run**: `$env:SMOKE_HEADLESS="false"; .\dialtone www smoke`


> [!CRITICAL]
> **Issue**: All screenshots are still images of the Earth (Home Section).
> Despite the `isProgrammaticScroll` timeout fix, the user reports that the browser is still snapping back to the home section before capture. The next agent must prioritize investigating why the scroll position is not persisting during the 2000ms wait.

**Status**: ⚠️ Partial Success (Logs OK, Visuals Broken)

## Context
We have overhauled the `www` smoke test to resolve screenshot snapback issues, enhance logging visibility, and standardize report generation (`SMOKE.md`).

## Key Changes
1.  **attempted Snapback Fix**:
    -   **File**: `src/plugins/www/app/src/main.ts`
    -   **Change**: Increased `isProgrammaticScroll` timeout from **1500ms** to **3000ms**.
    -   **Why**: To prevent the `IntersectionObserver` from re-activating and resetting scroll position during the 2000ms smoke test capture wait.

2.  **Enhanced Logging**:
    -   **Main.ts**: Logs `[main] 🔁 SWAP: #<id>` (Magenta) when switching sections.
    -   **Smoke.go**: Injects `[PROOFOFLIFE] 📸 SCREENSHOT STARTING: <section>` (Cyan) right before capture.
    -   **Filtering**: `smoke.go` now correctly filters these info logs from the error report while displaying them in the terminal.

3.  **Standardized Reporting**:
    -   **SMOKE.md**: Now includes a **5-Layer Colored DAG** with a Legend Table (Foundation, Core, Features, QA, Release).
    -   **Standard**: `SMOKE_REPORTING_STANDARD.md` updated to match this high-fidelity format.

## Recent Logs & Observations (Headed Run)
User observed: *"I see the earth section starting up and unpausing for every screen shot."*
This suggests the view is snapping back to Home (Earth) right before capture.

**Console Logs:**
```text
[APP] "%c[main] 🔁 SWAP: #s-about" "color: #8b5cf6; font-weight: bold"
[APP] "[PROOFOFLIFE] 📸 SCREENSHOT STARTING: s-about"
[TEST] Verify: hash=#s-about, scrollY=0, heap=0.0MB
```
*Note: `scrollY=0` is expected because `body` is the scroll container, not `window`. We need to check `document.body.scrollTop`.*

## Known Risks & Maintenance
-   **Timeout Coupling**: The **3000ms** timeout in `main.ts` must always exceed the **2000ms** wait in `smoke.go`. If wait times increase (e.g., for heavy 3D assets), update `main.ts` accordingly.
-   **Observer Thresholds**: Verify `threshold: [0.5, 0.75, 1]` still works if very tall sections are added.

## Next Steps
-   Apply the `SMOKE_REPORTING_STANDARD.md` to other plugins (e.g., `rover`).
-   Monitor CI/CD runs to ensure the 3000ms timeout is sufficient for slower runners.
