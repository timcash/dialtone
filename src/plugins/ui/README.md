# UIv2 Library

`src/plugins/ui` is the shared section shell used by plugin `src_vN/ui` apps.

```sh
# Run the canonical ui src_v1 suite (build fixture + Go backend + chromedp steps)
./dialtone.sh ui test src_v1

# Stream suite logs
./dialtone.sh logs stream --topic 'logs.test.ui.src-v1.>'
./dialtone.sh logs stream --topic 'logfilter.tag.pass.test'
./dialtone.sh logs stream --topic 'logfilter.tag.fail.test'
```

This README follows DAG `src_v3` terminology as source-of-truth.

## CLI

```sh
./dialtone.sh ui help
./dialtone.sh ui test src_v1
./dialtone.sh ui dev
./dialtone.sh ui build
./dialtone.sh ui install
```

`ui test src_v1` runs:
- `src/plugins/ui/src_v1/test/cmd/main.go`

The suite uses:
- `src/plugins/test/src_v1/go` (`StepContext`, suite orchestration, waits)
- `src/plugins/logs/src_v1/go` (structured logs + NATS topics)
- `chromedp` via test plugin browser helpers

Test folders:
- `src/plugins/ui/src_v1/test/01_build_and_serve`: Bun build + Go static backend + initial section readiness
- `src/plugins/ui/src_v1/test/02_sections_navigation`: menu-driven section navigation checks
- `src/plugins/ui/src_v1/test/03_component_actions`: table refresh, stage add, log input/enter interaction checks

## CSS Guide

This guide defines the standard structural classes and layout patterns for UI V2 sections.

### Structural Classes

UI V2 provides standard classes in `style.css` to ensure consistent layout across plugins:

- **`.overlay-primary`** (Underlay):
  - Fills the entire section (`width: 100%`, `height: 100%`).
  - Used for the main content: 3D Canvas, Video, Map, etc.
  - Usage: `<canvas class="hero-stage overlay-primary"></canvas>`

- **`.mode-form`** (Controls):
  - Defines the 3x4 grid of thumb controls.
  - Positioned absolutely at the bottom-center of the screen by default.
  - Usage: `<form class="mode-form" data-mode-form="...">`

- **`.overlay-legend`** (Info):
  - Positioned absolutely at the top-left (safe-area aware).
  - Used for HUDs, status text, and legends.
  - Usage: `<aside class="overlay-legend">...</aside>`

### Layout Modes

1.  **Fullscreen (Default)**:
    - The `.overlay-primary` fills the screen.
    - The `.mode-form` and `.overlay-legend` float on top (z-index).
    - Best for: 3D scenes, Maps, Video feeds.

2.  **Calculator (Split)**:
    - The screen is split vertically.
    - The `.mode-form` sits at the bottom (relative positioning).
    - The content (`.overlay-primary`) fills the remaining space above.
    - **Implementation**: Requires a specific CSS override for the section to change `.mode-form` to `position: relative` or `grid-row: 2`.
    - Best for: Data tables, Lists, Terminal logs where content shouldn't be obscured.

### Best Practices

- **Minimal Nesting**: Avoid deep selector chains. Use structural classes.
- **No Aria Selectors**: Do not use `[aria-label="..."]` for CSS styling. Use classes. Keep aria-labels for accessibility and testing only.
- **Mobile First**: All standard classes include mobile-responsive adjustments (e.g. larger touch targets).

## Core Model

- A UI has many `section`.
- A `section` is composed as:
  - one underlay
  - zero or more overlays

Section formula: `underlay + overlays = section`.

## Underlays

Exactly one underlay per section:

- `stage`
- `table`
- `docs`
- `xterm`
- `video`

## Overlays

Shared overlay kinds:

- `menu` (global)
- `mode-form`
- `legend`
- `chatlog` (optional)
- `status-bar` (optional)

`status-bar` is a first-class overlay in `ui.ts` via `UI_OVERLAYS.statusBar`.

## Section Naming Rule

Use:

- `<plugin-name>-<subname>-<underlay-type>`

Examples:

- `dag-meta-table`
- `dag-3d-stage`
- `dag-log-xterm`

## Section Registration

`SectionOverlayConfig` in `types.ts` supports:

- `primaryKind` and `primary` (required underlay binding)
- `modeForm` (preferred control overlay selector)
- `thumb` (deprecated alias of `modeForm`, kept for compatibility)
- `legend`
- `chatlog`
- `statusBar`

Example:

```ts
sections.register('dag-3d-stage', {
  containerId: 'dag-3d-stage',
  load: async () => mountStage(),
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Three Canvas']",
    modeForm: "form[data-mode-form='dag']",
    legend: '.dag-history',
    chatlog: '.dag-chatlog',
    statusBar: '.dag-status-bar',
  },
});
```

## Runtime Overlay Attributes

When an overlay selector resolves, `SectionManager` applies:

- `data-overlay="<kind>"`
- `data-overlay-role="<role>"`
- `data-overlay-section="<section-id>"`
- `data-overlay-active="true|false"`

Roles tracked by `SectionManager`:

- `primary`
- `mode-form`
- `legend`
- `chatlog`
- `status-bar`

## Menu Behavior

- `Menu` is the global overlay and uses `nav` as the modal root.
- On open, menu hides active `mode-form` overlays (`data-overlay='mode-form'`).
- Legacy `thumb` overlay hide rule is still supported for older sections.
