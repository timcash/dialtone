# Plugin: www

live website at [dialtone.earth](https://dialtone.earth).

## Workflow (local dev → section → publish)

### 1) Start dev server

```shell
./dialtone.sh www dev
```

Optional quick openers:

```shell
./dialtone.sh www about demo
./dialtone.sh www radio demo
./dialtone.sh www cad demo
./dialtone.sh www earth demo
./dialtone.sh www webgpu demo
```

### 2) Create or edit a section

Minimal section recipe (Three.js or WebGPU):

1. Add the section markup in `app/index.html`:
   - `<section id="s-yourid" class="snap-slide">`
   - `<div id="yourid-container"></div>`
2. Add container CSS in `app/style.css`:
   - `#yourid-container { position: absolute; inset: 0; }`
   - `#yourid-container canvas { display: block; }`
3. Register the section in `app/src/main.ts`:
   - `sections.register('s-yourid', { containerId: 'yourid-container', load: async () => mountYour(container) });`
4. Implement `mountYour()` in `app/src/components/*`:
   - Return `{ dispose, setVisible }`
   - Respect `VisibilityMixin` so animation pauses when off-screen

Use these templates as starting points:

- **Three.js template:** `app/src/components/threejs-template.ts` (`s-threejs-template`)
- **WebGPU template:** `app/src/components/webgpu-template.ts` (`s-webgpu-template`)

### 3) Config panel pattern (standard layout)

All config sliders use the same grid layout:

- `.earth-config-row` (3-column grid: label / slider / value)
- `.earth-config-label` for left-aligned labels

To add a config panel:

1. Add `<div id="yourid-config-panel" class="earth-config-panel" hidden></div>` to the section markup.
2. Add a toggle button into `.top-right-controls` (class `earth-config-toggle`).
3. Show the toggle when the section is visible in `app/style.css` via:
   - `.snap-slide.is-visible[id="s-yourid"]~.top-right-controls #yourid-config-toggle`

Examples:

- Earth config: `app/src/components/earth/config_ui.ts`
- About config: `app/src/components/about.ts`
- Template configs: `app/src/components/threejs-template.ts`, `webgpu-template.ts`

### 4) Earth land layer (GeoJSON → H3)

The Earth section prefers a precomputed H3 layer:

- `app/public/land.h3.json` (loaded first)
- Falls back to `app/public/land.geojson` if missing

To regenerate:

```shell
cd src/plugins/www/app
node scripts/build_land_h3.cjs 3
```

### 5) Publish

```shell
./dialtone.sh www publish
```

This bumps the version, builds, and deploys to Vercel.

## Where things live

- `app/index.html` — section markup + version tag
- `app/src/main.ts` — section registration + lazy loading
- `app/style.css` — global layout + config panel styles
- `app/src/components/*` — section visuals + UI
- `app/src/shaders/*` — GLSL for Three.js sections
- `app/src/components/typing.ts` — subtitle typing defaults

## Vercel config

```shell
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7
```
