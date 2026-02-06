# Plugin: www

Vercel wrapper for the public website at [dialtone.earth](https://dialtone.earth).

## Simplest working section: `threejs-template`

The **threejs-template** component is the minimal example of a working Three.js section. Use it as the starting point for new sections.

- **Component:** `app/src/components/threejs-template.ts`
- **Section ID:** `s-threejs-template` · **Container:** `#threejs-template-container`
- **Scene:** One cube in the center, camera facing it, key light and soft glow shader. Implements `VisualizationControl` (dispose, setVisible) and uses `VisibilityMixin` so the section manager can lazy-load and pause when off-screen.

To add a section like this:

1. Add a `<section id="s-yourid" class="snap-slide">` with `<div id="yourid-container"></div>` in `index.html`.
2. Add `#yourid-container` (and `#yourid-container canvas`) to the visualization container rules in `style.css`.
3. Register in `main.ts`: `sections.register('s-yourid', { containerId: 'yourid-container', load: async () => { ... mountYour(container) ... } });`
4. Implement a mount function that returns `{ dispose, setVisible }` and a visualization that respects `setVisible` in its animation loop.

See `threejs-template.ts` and `section.ts` for the contract.

## WebGPU template: `webgpu-template`

The **webgpu-template** component is the minimal example of a working WebGPU section (no Three.js). Use it as the starting point for new WebGPU-based sections.

- **Component:** `app/src/components/webgpu-template.ts`
- **Section ID:** `s-webgpu-template` · **Container:** `#webgpu-template-container`
- **Scene:** Lit sphere, WGSL shaders, adapter/device/context setup, depth buffer, uniform buffer, and the same `{ dispose, setVisible }` contract as other sections.

Run `./dialtone.sh www webgpu demo` to open the WebGPU template section. When WebGPU is unavailable, the section shows a fallback message instead of a canvas.

## Folder Structure

```shell
src/plugins/www/
├── cli/
│   └── www.go           # CLI commands and Vercel integration
├── app/
│   ├── index.html       # Landing page with version tag
│   ├── package.json     # Version and dependencies
│   ├── vite.config.mjs  # Vite build config
│   ├── vercel.json      
│   └── src/
│       ├── main.ts
│       ├── components/  # Earth, neural network, etc.
│       └── shaders/     # GLSL shaders
└── README.md
```

## Command Line Help

```shell
./dialtone.sh www dev              # Start Vite dev server
./dialtone.sh www build            # Run local production build
./dialtone.sh www publish          # Bump version + build + deploy
./dialtone.sh www validate         # Check deployed version matches local
./dialtone.sh www logs <url>       # View deployment logs
./dialtone.sh www domain [url]     # Alias deployment to dialtone.earth
./dialtone.sh www login            # Login to Vercel
./dialtone.sh www radio demo       # Dev server + Chrome on Radio section (#s-radio)
./dialtone.sh www cad demo         # CAD backend + dev server + Chrome on CAD section
./dialtone.sh www earth demo       # Dev server + Chrome on Earth section
./dialtone.sh www webgpu demo      # Dev server + Chrome on WebGPU section
```

## Sections

- **s-threejs-template** — Three.js template (cube + key light + glow); use as example for new sections
- **s-home** — Earth visualization
- **s-robot** — Robot arm IK
- **s-neural** — Neural network
- **s-math** — Math / geometry
- **s-cad** — Parametric CAD (gear)
- **s-webgpu-template** — WebGPU template (lit sphere; use as example for WebGPU sections)
- **s-radio** — Open-source handheld radio (Three.js template)
- **s-about** — About dialtone
- **s-docs** — Documentation

## Publish Workflow

```shell
# What ./dialtone.sh www publish does:
# 1. Bump patch version in package.json (1.0.9 → 1.0.10)
# 2. Update version tag in index.html (<p class="version">v1.0.10</p>)
# 3. Run npm run build (Vite production build)
# 4. Run vercel build --prod (create prebuilt output)
# 5. Run vercel deploy --prebuilt --prod
```

## Vercel Configuration

```shell
# Hardcoded in www.go:
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7

# Project name in Vercel dashboard: app
# Domain: dialtone.earth
```

## Validation

```shell
# Verify deployed version matches local package.json
./dialtone.sh www validate

# Output:
# [www] Version OK: site=v1.0.9 expected=v1.0.9
```
