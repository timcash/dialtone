# Radio component

Three.js section that shows an open-source handheld radio for small robots. Built on the same stack as **threejs-template** (key light, glow shader, FPS, backend label, marketing overlay).

## Location

- **Component:** `../radio.ts` (parent folder)
- **Section ID:** `s-radio` · **Container:** `#radio-container`
- **URL hash:** `#s-radio`

## Scene

- **Camera:** Perspective at `(0, 0, 3)` looking at origin.
- **Background:** Black (`0x000000`).
- **Animation:** Radio group has a slow oscillation (`rotation.y` and `rotation.x` with sin(time)); lights are fixed.

## Lighting

- **Ambient:** `0xccddff`, intensity `0.55`.
- **Key (warm):** `0xffddbb`, intensity `1.8`, position `(4, 3, 2.5)` — drives body glow shader `uLightDir`.
- **Rim (cool):** `0x88aaff`, intensity `1.2`, position `(-2.5, 1, -4)`.
- **Front:** White, intensity `0.9`, position `(0, 0, 5)` — keeps LCD and knobs visible.

## Model

Built from four groups (all under `radioGroup`):

| Group         | Contents                                      |
|---------------|-----------------------------------------------|
| `bodyGroup`   | Single box, custom `ShaderMaterial` (template-cube glow: `uColor` `0x5a6070`, `uGlowColor` `0x88aacc`) |
| `lcdGroup`    | Plane for LCD; `MeshStandardMaterial` green emissive |
| `antennasGroup` | Two cylinders (left/right), metal/rough       |
| `knobsGroup`  | Two cylinders (knobs on front), metal/rough   |

Approximate size: body 1.2×0.6×0.25; LCD 0.5×0.22; antennas ~0.35 tall; knobs on +Z face.

## Shaders

Body uses **template-cube** glow shaders:

- `src/plugins/www/app/src/shaders/template-cube.vert.glsl`
- `src/plugins/www/app/src/shaders/template-cube.frag.glsl`

Uniforms: `uColor`, `uGlowColor`, `uLightDir` (view space), `uTime`.

## Contract

- **Mount:** `mountRadio(container)` returns `{ dispose, setVisible }`.
- **Visibility:** Uses `VisibilityMixin`; animation loop skips work when section is off-screen.
- **UI:** Marketing overlay (heading + copy) and backend label (`data-radio-backend`) for “Rendering: WebGL 2 · WebGPU: …”.

## How to run

```bash
./dialtone.sh www dev
# Open http://localhost:<port>/#s-radio

./dialtone.sh www radio demo   # Dev server + Chrome on #s-radio
```

## Adding to the site

The section is registered in `main.ts` and the container/canvas are styled in `style.css`. See the www plugin README (“Simplest working section”) for the pattern used to add sections like this.
