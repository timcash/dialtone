# UIv2 Library

`src/libs/ui_v2` is the shared section shell for Dialtone plugin UIs.

## Design Language

Each plugin `src_vN` has one `ui`.

Each `ui` can have many `section`.

Each `section` is composed from underlays + overlays:

- underlays (exactly one per section): `stage` | `table` | `docs` | `xterm` | `video`
- overlays (shared UI layers): `menu` (global), `thumbs`, `legend`, optional `chatlog`

## Menu Overlay Behavior

- `Menu` renders a fullscreen modal.
- Modal content uses CSS grid for menu buttons.
- Menu grid has an effective minimum width target of `400px` (bounded by viewport width).
- While menu is open, active-section `thumbs` overlays are hidden automatically.

## Section Registration

`SectionConfig` supports section layer selectors:

```ts
sections.register('my-section', {
  containerId: 'my-section',
  load: async () => mountMySection(),
  overlays: {
    primaryKind: 'stage',
    primary: '.my-stage',
    thumb: '.my-thumbs',
    legend: '.my-legend',
    chatlog: '.my-chatlog',
  },
});
```

Notes:
- `primaryKind` + `primary` represent the section underlay in the runtime API.
- `thumb` selector points to the thumbs overlay container.

`SectionManager` tags matched elements with:

- `data-overlay="<kind>"`
- `data-overlay-role="primary|thumb|legend|chatlog"`
- `data-overlay-section="<section-id>"`
- `data-overlay-active="true|false"`

Sections load dynamically from `load()`. If a section is not cached yet, `ui_v2` shows a quick loading overlay during first load.
