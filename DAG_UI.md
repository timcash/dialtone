# DAG UI Standardization & Updates

This document tracks the required architectural and stylistic updates for the `dag` src_v3 UI to align with the core Dialtone `ui` src_v1 standards.

## 1. Layout Standardization
*   **Grid vs. Flex**: Migrate the top-level section containers from `display: flex` or `position: relative` to the standard `display: grid`.
    *   **Row 1**: `.overlay-primary` (Canvas, Table, or Terminal).
    *   **Row 2**: `.mode-form` (Thumb controls).
*   **Remove Absolute Positioning**: Eliminate `position: absolute` on the `mode-form` in the 3D Stage and Log sections. Use `grid-row: 2` to ensure the form is consistently placed at the bottom of the viewport without overlapping primary content.

## 2. CSS Inheritance & Cleanup
*   **Inherit Global Styles**: Remove local definitions for `button`, `input`, and `.overlay-legend`. These are now provided by `@ui/style.css`.
*   **Variable Usage**: Transition local color and spacing definitions to use the global theme variables (e.g., `--theme-primary`, `--theme-bg`).
*   **Redundant Overlays**: Remove local z-index and visibility logic for the Global Menu; this is now managed by the core `Menu.ts`.

## 3. Component Lifecycle Coordination
*   **SectionManager Integration**: Ensure all visualization components (Table, Three, Log) implement the `VisualizationControl` interface correctly.
*   **Resize Handling**: Use the `setVisible(true)` hook to trigger `renderer.setSize` for Three.js and `fit.fit()` for Xterm.js. This ensures layout is correct after section switches or hash-based navigation.

## 4. Specific DAG UI Enhancements
*   **Chatlog Placement**: Standardize the `.dag-chatlog` as an overlay within the 5th row of the `.mode-form` grid to prevent it from floating over 3D nodes.
*   **Viewport Constraints**: Update the Three.js canvas to use `100%` height/width within its grid cell rather than `100vh/100vw` to prevent it from bleeding under the form.
*   **Table Scroll**: Ensure the `#dag-meta-table` uses `min-height: 0` and `overflow-y: auto` on its wrapper to prevent the entire page from scrolling.

## 5. Mobile & Touch Alignment
*   **Safe Areas**: Ensure the bottom margin of the `mode-form` accounts for `env(safe-area-inset-bottom)` consistently across all sections.
*   **Touch Action**: Standardize `touch-action: manipulation` on all interactive canvas and button elements to prevent accidental zooming during fast interactions.
