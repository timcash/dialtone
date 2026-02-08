# Plugin: www

live website at [dialtone.earth](https://dialtone.earth).

## Workflow (local dev → section → publish)

### 1) Start dev server

```shell
.\dialtone www dev
.\dialtone www smoke
.\dialtone www test
.\dialtone www publish
```

Optional quick openers:

```shell
.\dialtone www about demo
.\dialtone www radio demo
.\dialtone www cad demo
.\dialtone www earth demo
.\dialtone www webgpu demo
```


### 4) Earth land layer (GeoJSON → H3)

The Earth section prefers a precomputed H3 layer:

- `app/public/land.h3.json` (loaded first)
- Falls back to `app/public/land.geojson` if missing

To regenerate:

```shell
cd src/plugins/www/app
node scripts/build_land_h3.cjs 3
```



## Vercel config

```shell
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7
```

# Log

## 2026-02-07T12:04:42-08:00

Fixed a visual bug where clouds disappeared when passing over land.

-   **Issue**: Both the Land and Cloud layers use transparency (`depthWrite: false`) to prevent self-occlusion artifacts. However, without depth writing, the GPU relies on draw order. The default draw order caused the Land (renderOrder 1) to be drawn *after* the Clouds (default 0), overwriting them.
-   **Fix**: Explicitly set `renderOrder = 2` for both cloud layers in `components/earth/index.ts`. This ensures clouds are drawn last, appearing on top of the land.
-   **Update**: Adjusted default camera position (distance: 23.5, orbit: 5.74, yaw: 0.99) per user feedback.

## 2026-02-06T17:41:34-08:00

Migrated the entire application to use a unified `Menu` utility class for configuration panels.

-   **Created `util/menu.ts`**: Centralized UI logic for sliders, buttons, and layout.
-   **Refactored Components**: Migrated all components (`threejs-template`, `webgpu-template`, `cad`, `earth`, `math`, `robot`, `geotools`, `nn`, `about`) to use the new `Menu` system.
-   **Cleanup**: Removed legacy `config.ts`, `config_ui.ts`, `config_behavior.ts`, and all `earth-config-panel` HTML injection points.
-   **Style**: Fixed CSS syntax errors and optimized mobile font sizes for marketing overlays.
 files.
-   **Styling**: Enforced consistent styling via `.menu-panel` and related classes.
