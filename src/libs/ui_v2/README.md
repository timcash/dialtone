# UIv2 Library

`src/libs/ui_v2` is the shared section shell for Dialtone plugin UIs.

## Design Language

Each plugin `src_vN` has one `ui`.

Each `ui` can have many `section`.

Each `section` uses four overlays:

- `menu` (global): section switcher overlay.
- `thumb`: interactive controls (buttons, inputs).
- `legend`: non-interactive context (stats, logs, history).
- one primary overlay kind: `stage` or `table` or `xterm` or `docs` (more kinds can be added later).

## Menu Overlay Behavior

- `Menu` renders a fullscreen modal.
- Modal content uses CSS grid for menu buttons.
- Menu grid has an effective minimum width target of `400px` (bounded by viewport width).
- While menu is open, active-section `thumb` overlays are hidden automatically.

## Section Registration

`SectionConfig` now supports overlay selectors:

```ts
sections.register('my-section', {
  containerId: 'my-section',
  load: async () => mountMySection(),
  overlays: {
    primaryKind: 'stage',
    primary: '.my-stage',
    thumb: '.my-thumb',
    legend: '.my-legend',
  },
});
```

`SectionManager` tags matched elements with:

- `data-overlay="<kind>"`
- `data-overlay-role="primary|thumb|legend"`
- `data-overlay-section="<section-id>"`
- `data-overlay-active="true|false"`

Sections load dynamically from `load()`. If a section is not cached yet, `ui_v2` shows a quick loading overlay during first load.
