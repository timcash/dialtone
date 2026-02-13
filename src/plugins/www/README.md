# Plugin: www

> **Agent Note**: Use `./dialtone.sh www` on macOS/Linux and `.\dialtone.ps1 www` on Windows to interact with this plugin.

![Dialtone WWW Summary](screenshots/summary.png)

Live website at [dialtone.earth](https://dialtone.earth).

## Agent Guide: Working with WWW

This plugin manages the Dialtone public website, a high-performance Three.js application built with Vite and TypeScript. It uses a "snap-slide" architecture where each section is a lazy-loaded Three.js component.

### 1. Development Workflows

#### Launching Demos (Preferred for Agents)
Demos orchestrate a local dev server and a Chrome instance locked to a specific section with GPU acceleration and console log forwarding.

```powershell
# Windows
.\dialtone.ps1 www vision demo
.\dialtone.ps1 www earth demo
.\dialtone.ps1 www music demo
.\dialtone.ps1 www radio demo
.\dialtone.ps1 www cad demo
```

```bash
# macOS/Linux
./dialtone.sh www vision demo
./dialtone.sh www earth demo
# ... etc
```

#### Standard Vite Workflow
```powershell
.\dialtone.ps1 www dev      # Start dev server
.\dialtone.ps1 www build    # Production build
.\dialtone.ps1 www publish  # Deploy to Vercel (Production)
```

### 2. Architecture & Components

The application core is located in `app/src/`.

- **Main Entry**: `app/src/main.ts` handles section registration and global event listeners (scroll, swipe, hash changes).
- **Section Manager**: `app/src/components/util/section.ts` manages lazy loading and pauses/resumes animations based on visibility to save resources.
- **Menu System**: `app/src/components/util/menu.ts` provides a unified UI for sliders and buttons.
- **Styles**: `app/style.css` contains global layout, snap-scroll logic, and marketing overlay animations.

#### Creating a New Section
1. Create a folder in `app/src/components/<name>/`.
2. Implement a `mount<Name>(container: HTMLElement)` function that returns a `VisualizationControl`.
3. Register the section in `app/src/main.ts`.
4. Add a `<section id="s-<name>">` block in `app/index.html`.

### 3. Verification & Testing

#### Smoke Test
Always run a smoke test before publishing. It captures screenshots and performance metrics (FPS, GPU/CPU time) for every section.

```powershell
.\dialtone.ps1 www smoke
```

Reports are generated at `src/plugins/www/SMOKE.md`.

### 4. Specialized Tooling

#### Earth Land Layer (H3)
The Earth section uses precomputed H3 hexagonal grids. To regenerate from GeoJSON:
```bash
cd src/plugins/www/app
node scripts/build_land_h3.cjs 3
```

## Vercel Config
```shell
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7
```

# Log

## 2026-02-13
- Added `vision` component with real-time 3D pose estimation.
- Integrated `tinygesture` for mobile swipe navigation.
- Moved marketing text to top-left and menu to bottom-right.
- Added `www vision demo` CLI command.

## 2026-02-07
- Fixed cloud transparency/renderOrder bug in Earth section.

## 2026-02-06
- Migrated to unified `Menu` utility in `util/menu.ts`.
