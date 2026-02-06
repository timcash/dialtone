# WWW Plugin Update Summary (for next LLM)

## What was done
- Added a smoke test at `src/plugins/www/test/smoke.go` using chromedp.
  - Starts dev server if needed, attaches to Chrome debug port or launches Chrome.
  - Navigates `/#s-*` sections, waits 0.5s, fails on console warnings/errors.
  - Logs each step with clear `>> [WWW] Smoke:` messages.
  - Test is registered as `www-smoke` with tags `www, smoke, browser`.
- Added CLI entrypoint `./dialtone.sh www smoke` in `src/plugins/www/cli/www.go`.
  - Runs `./dialtone.sh test tags smoke`.
- Reorganized `app/src/components/` into per-component folders and moved files with `mv`.
  - `components/<name>/index.ts` and `components/<name>/config.ts` for each section.
  - Shared utilities remain in `components/` root (e.g., `fps.ts`, `gpu_timer.ts`, `section.ts`, `typing.ts`).
  - Earth helpers live in `components/earth/` (already existing).
  - About helpers moved into `components/about/` (`arc_renderer.ts`, `search_lights.ts`, `vision_grid.ts`).
  - Earth `hex_layer.ts` moved into `components/earth/hex_layer.ts`.
- Updated imports to new paths:
  - Section loaders in `app/src/main.ts` now import `./components/<name>/index`.
  - Shader import paths updated to `../../shaders/...` in nested folders.
  - Config imports updated to `./config` within each component folder.

## Current failure observed
- `./dialtone.sh www smoke` fails while navigating:
  - `Failed to fetch dynamically imported module: http://127.0.0.1:5173/src/components/robot/index.ts`
- `curl http://127.0.0.1:5173/src/components/robot/index.ts` returns HTTP 500.
  - Likely Vite import/path error due to refactor.

## Latest smoke run (2026-02-06)
- `./dialtone.sh www smoke` still fails at `#s-robot`.
- Error: `TypeError: Failed to fetch dynamically imported module: http://127.0.0.1:5173/src/components/robot/index.ts`
- Smoke log shows dev server OK, Chrome launched, `#s-home` OK, `#s-robot` fails.

## Next steps
1. Fix Vite module resolution errors:
   - Check Vite dev server logs for exact error (500 on `/src/components/robot/index.ts`).
   - Verify all imports in `components/robot/index.ts` now point to correct paths.
   - Repeat for other moved components if needed.
2. Re-run `./dialtone.sh www smoke` until all sections pass.
3. If needed, remove old commented config code blocks left in:
   - `components/math/index.ts`, `components/nn/index.ts`, `components/robot/index.ts`
4. Consider adding a short note in README about new component folder layout.

## Key files changed
- `src/plugins/www/test/smoke.go`
- `src/plugins/www/cli/www.go`
- `src/plugins/www/app/src/main.ts`
- `src/plugins/www/app/src/components/**` (moved into folders)
