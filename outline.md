# Summary for Next Agent

## Status
- `https://dialtone.earth` and `https://www.dialtone.earth` now serve `v1.0.2`.
- Latest production deployment: `https://dialtone-earth-pprm19113-timcashs-projects.vercel.app`.
- Root cause was `dialtone-earth` project builds failing and deploys targeting the wrong Vercel project.

## What changed (key files)
- Earth scene refactor: `src/plugins/www/app/src/components/earth.ts`
  - Oscillating time scale, orbit height, HUD telemetry updates.
  - Independent cloud motion; extra cloud layers; icy cloud shader with glow.
  - Sun/key light orbits faster (`sunOrbitSpeed = 0.01`) and stays opposite ISS camera direction.
- Shaders moved to files in `src/plugins/www/app/src/shaders/`:
  - `earth.vert.glsl`, `earth.frag.glsl` (land darker green)
  - `cloud.vert.glsl`, `cloud.frag.glsl`
  - `cloud_ice.frag.glsl` (blue glow)
  - `atmosphere.vert.glsl`, `atmosphere.frag.glsl`
  - `sun_atmosphere.vert.glsl`, `sun_atmosphere.frag.glsl`
- Added shader import typing: `src/plugins/www/app/src/vite-env.d.ts`
- Added `.vercelignore` to shrink deploy upload.
- Added `@types/three` dev dependency in `src/plugins/www/app`.
- Added `www publish-prebuilt` command in `src/plugins/www/cli/www.go` and documented in `src/plugins/www/README.md`.
- `globe.gl` removed from www app (component deleted, deps removed).

## Publish flow used
1. Linked repo root to `dialtone-earth` project (`rootDirectory` is `src/plugins/www/app`).
2. `vercel build --prod` from repo root (builds `src/plugins/www/app`).
3. `vercel deploy --prebuilt --prod` (auto-aliased `www.dialtone.earth` and `dialtone.earth`).

## Verification checks
- Curl version tag:
  - `https://dialtone.earth` returns `v1.0.2`
  - `https://www.dialtone.earth` returns `v1.0.2`
- Version tag parsing used:
  - `curl -s -L -A "Mozilla/5.0" https://dialtone.earth | python3 -c "import re,sys;html=sys.stdin.read();print(re.search(r'class=\\\\\"version\\\\\">([^<]+)<',html).group(1) if re.search(r'class=\\\\\"version\\\\\">([^<]+)<',html) else 'no-version-tag')"`

## Open tasks / next steps
- Restore root `.vercel/project.json` to `dialtone` if needed for other deploys.
- Optionally update ticket `earth-www-hud` to `done`.
