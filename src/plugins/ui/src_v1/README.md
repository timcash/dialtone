# UI src_v1 Library

## Runtime Note

Plain `./dialtone.sh ui src_v1 test ...` is the default operator path.

That command is normally routed through the local REPL leader, which means:
- `DIALTONE>` should stay high-level
- detailed test output stays in the subtone log
- use `./dialtone.sh repl src_v3 subtone-list --count 20`
- use `./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200`

For WSL, the default remote headed browser path now targets `legion` through `chrome src_v3`.

Typical test summaries are:

```text
DIALTONE> ui test: preparing remote chrome session on legion
DIALTONE> ui test: ensuring chrome src_v3 role=dev on legion
DIALTONE> ui test: running 1 suite steps
DIALTONE> ui test: suite passed
```

```sh
# generic plugin workflow
./dialtone.sh ui src_v1 install
./dialtone.sh ui src_v1 fmt
./dialtone.sh ui src_v1 fmt-check
./dialtone.sh ui src_v1 lint
./dialtone.sh ui src_v1 build
./dialtone.sh ui src_v1 dev
./dialtone.sh ui src_v1 test

# useful variants
# Run the full suite via the local REPL path.
./dialtone.sh ui src_v1 test

# Run one headed attach test against the managed Chrome v3 daemon on legion.
./dialtone.sh ui src_v1 dev --browser-node legion
./dialtone.sh ui src_v1 test --attach legion
./dialtone.sh ui src_v1 test --attach legion --filter ui-build-and-go-serve

# Inspect the subtone log if the high-level transcript is not enough.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

`src/plugins/ui/src_v1/ui` is the shared section shell used by plugin UIs (robot, earth, fixture apps).

## Getting Started

1. Import the shared app shell:

```ts
import { setupApp } from '@ui/ui';
```

2. Create the app:

```ts
const { sections, menu } = setupApp({ title: 'dialtone.myplugin', debug: true });
```

PWA support (from the UI library):

```ts
const { sections, menu } = setupApp({
  title: 'dialtone.myplugin',
  debug: true,
  pwa: {
    enabled: true,
    serviceWorkerPath: '/sw.js',
    disableInDev: false,
  },
});
```

`setupApp` registers the service worker when enabled. Keep your plugin `index.html` linking `manifest.webmanifest`.

3. Register sections with the shared starter-shell helper:

```ts
import { registerUISharedSections } from '@ui/templates';

registerUISharedSections({
  sections,
  menu,
  entries: [
    { sectionID: 'myplugin-home-docs', template: 'docs', title: 'Overview' },
    { sectionID: 'myplugin-runs-table', template: 'table', title: 'Runs' },
    { sectionID: 'myplugin-log-terminal', template: 'terminal', title: 'Signals' },
  ],
  decorate: (entry, container) => {
    // optional per-section enhancement after the shell is rendered
  },
});
```

The helper renders the shell, binds overlays, and adds menu items.

4. If you need a custom section, you can still register it directly:

```ts
sections.register('myplugin-three-stage', {
  containerId: 'myplugin-three-stage',
  canonicalName: 'myplugin-three-stage',
  load: async () => mountThree(document.getElementById('myplugin-three-stage')!),
  overlays: {
    primaryKind: 'three',
    primary: '.three-stage',
    form: '.mode-form',
    legend: '.overlay-legend',
    chatlog: '.overlay-chatlog',
  },
});
```

## Naming Contract

Use these names consistently across all plugins.

- Section ID format:
  - `<plugin>-<subname>-<underlay-type>`
  - examples: `robot-three-stage`, `robot-table-table`, `test-signals-terminal`
- Underlay terminology:
  - exactly one underlay per section
  - canonical `primaryKind` values: `three | table | terminal | docs | camera`
  - compatibility aliases still work: `stage -> three`, `xterm -> terminal`, `video -> camera`, `button-list -> settings`
- Overlay terminology:
  - `form` (preferred key in config; `modeForm` still supported)
  - `legend` (header/legend overlay selector)
  - optional: `chatlog`, `statusBar`
- CSS class conventions:
  - underlay classes: `three-stage`, `table-wrapper`, `xterm-primary`, `docs-primary`, `camera-stage`
  - overlay classes: `mode-form`, `overlay-legend`, optional `overlay-chatlog`, `overlay-status-bar`
- `data-mode-form`:
  - starter shells do not require a custom value
  - if you set it manually, use the full section ID

## Underlay/Overlay Rules

- One section = one primary underlay.
- Keep control grids in `form` overlays, not mixed into underlay markup.
- Keep legend/status/chatlog layers in overlays so visibility and active state are managed uniformly by `SectionManager`.
- Mobile behavior should come from the shared shell first. Only add section-specific layout CSS when the shell is not enough.
- The form is expected to be toggleable via the built-in `Toggle Mode Form` button that `SectionManager` attaches.

## Shared Templates

Shared test/demo templates live in:
- `src/plugins/ui/src_v1/ui/templates.ts`

They are the intended starter shells for new plugins:
- `docs`
- `table`
- `three`
- `terminal`
- `camera`

## Testing Layout

The existing UI test suite already checks for overlay collisions.

- Mobile viewport helper:
  - `src/plugins/ui/src_v1/test/mobile_viewport.go`
- Overlay overlap detection:
  - `src/plugins/test/src_v1/go/overlap.go`
- Menu-navigation regression checks:
  - `src/plugins/ui/src_v1/test/sections_navigation_lib/run.go`

The overlap checker inspects active overlays/buttons and fails the test on unexpected intersections, except for the global menu modal while it is open.
