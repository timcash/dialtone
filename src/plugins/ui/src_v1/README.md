# UI src_v1 Library

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
./dialtone.sh ui src_v1 dev --browser-node legion
./dialtone.sh ui src_v1 test --attach legion
./dialtone.sh ui src_v1 test --filter ui-section-hero-via-menu
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

3. Register sections with the shared `SectionManager`:

```ts
sections.register('myplugin-hero-stage', {
  containerId: 'myplugin-hero-stage',
  canonicalName: 'myplugin-hero-stage',
  load: async () => {
    const container = document.getElementById('myplugin-hero-stage');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  overlays: {
    primaryKind: 'stage',
    primary: '.underlay-stage',
    form: '.mode-form',
    legend: '.overlay-legend',
  },
});
```

4. Add menu navigation:

```ts
menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('myplugin-hero-stage');
});
```

## Naming Contract

Use these names consistently across all plugins.

- Section ID format:
  - `<plugin>-<subname>-<underlay-type>`
  - examples: `robot-hero-stage`, `robot-table-table`, `earth-hero-stage`
- Underlay terminology:
  - exactly one underlay per section
  - set `primaryKind` to one of: `stage | table | xterm | docs | video | button-list`
- Overlay terminology:
  - `form` (preferred key in config; `modeForm` still supported)
  - `legend` (header/legend overlay selector)
  - optional: `chatlog`, `statusBar`
- CSS class conventions:
  - underlay classes: `underlay-stage`, `underlay-table`, `underlay-xterm`, `underlay-docs`, `underlay-video`, `underlay-button-list`
  - overlay classes: `mode-form`, `overlay-legend`, optional `overlay-chatlog`, `overlay-status-bar`
- `data-mode-form`:
  - use the full section ID (not legacy aliases like `log`)

## Underlay/Overlay Rules

- One section = one primary underlay.
- Keep control grids in `form` overlays, not mixed into underlay markup.
- Keep legend/status/chatlog layers in overlays so visibility and active state are managed uniformly by `SectionManager`.

## Shared Templates

Shared test/demo templates live in:
- `src/plugins/ui/src_v1/ui/templates.ts`

They follow the same underlay/overlay model and should be used as reference for new sections.
