# UI v1 Shared Library

`src/plugins/ui` is the shared UI shell for plugin web apps (`src_vN/ui`).

## CLI

```sh
./dialtone.sh ui src_v1 install
./dialtone.sh ui src_v1 build
./dialtone.sh ui src_v1 dev
./dialtone.sh ui src_v1 test
```

## Core Model

A section is composed as:

- one **underlay** (main content surface)
- zero or more **overlays** (controls and HUD layers)

Formula:

`section = underlay + overlays`

## Underlays

Underlays are the primary interaction/view surfaces. Typical underlays:

- `three-stage` (3D canvas)
- `table` (telemetry/data grid)
- `video-stage` (camera/media)
- `xterm` (terminal)
- `docs` (scrollable text/docs)

Shared classes (in `src/plugins/ui/src_v1/ui/style.css`):

- `.overlay-primary` (base underlay fill class)
- `.underlay-docs` / `.docs-primary`
- `.underlay-table` / `.table-wrapper` / `.telemetry-table`
- `.underlay-xterm` / `.xterm-primary`
- `.underlay-video` / `.video-stage`

## Overlays

Overlays are floating/control layers above underlays:

- `menu` (global nav)
- `mode-form` (thumb/control form)
- `legend` (HUD/information)
- `status-bar` (optional)
- `chatlog` (optional)
- `watchdog` (optional)

Shared classes/roles:

- `.mode-form`
- `.overlay-legend`
- `.overlay-chatlog` or `[data-overlay='chatlog']`
- `.overlay-status-bar` or `[data-overlay='status-bar']`
- `.overlay-watchdog`

## Layout Modes

UI sections support two layout modes:

1. `fullscreen`
- Underlay fills the full section.
- Overlays float on top.
- Use for 3D/video hero views.

2. `calculator`
- Underlay occupies row 1.
- Mode form/control overlay stays in row 2 (bottom control strip).
- Use for table/xterm/docs-like flows where content should remain visible above controls.

These are implemented in shared CSS via:

- `section.fullscreen`
- `section.calculator`

## Section Registration

Use `SectionManager` overlay config to bind underlay + overlays.

```ts
sections.register('robot-three-stage', {
  containerId: 'three',
  load: async () => mountThree(),
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Three Canvas']",
    modeForm: "form[data-mode-form='three']",
    legend: '.three-legend',
    chatlog: '.three-chatlog',
    statusBar: '.three-status',
  },
});
```

Runtime attributes added by `SectionManager`:

- `data-overlay="<kind>"`
- `data-overlay-role="<role>"`
- `data-overlay-section="<section-id>"`
- `data-overlay-active="true|false"`

## ARIA Labels (Testing Contract)

Use `aria-label` consistently for stable automation selectors.

Rules:

- Every section root gets an `aria-label` (e.g. `"Three Section"`).
- Every primary underlay gets an `aria-label` (e.g. `"Three Canvas"`, `"Video Stage"`, `"Xterm Terminal"`).
- Every mode-form button/input gets explicit `aria-label`.
- Keep ARIA labels stable across refactors; tests depend on them.

Style with classes, not ARIA selectors.
ARIA is for accessibility + test targeting only.

## Menu Behavior

- Menu is a global overlay.
- When menu is open, mode-form overlays are hidden (`data-overlay='mode-form'`, legacy `thumb` alias supported).

## Testing

Canonical suite:

- `src/plugins/ui/src_v1/test/cmd/main.go`

Run:

```sh
./dialtone.sh ui src_v1 test
./dialtone.sh logs src_v1 stream --topic 'logs.test.ui.src-v1.>'
```
