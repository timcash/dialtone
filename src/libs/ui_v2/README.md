# UIv2 Library

`src/libs/ui_v2` is the shared section shell used by plugin `src_vN/ui` apps.

This README follows DAG `src_v3` terminology as source-of-truth.

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
