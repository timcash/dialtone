# Chrome Plugin Current Tasks

## Goal
Make chrome control fully service-driven on remote hosts (no client-side chromedp for remote actions), keep one stable profile + one tab per role, and support multi-host CLI control.

## Completed
- Added `chrome open --host <host|all> --fullscreen --url ...` command flow.
- Added `chrome list --host <host|all>`:
  - shows host, role, headed/headless, debug port, active, tabs.
- Added service-side `/open` endpoint in chrome daemon.
- Added service-side `/action` endpoint in chrome daemon.
- Added new CLI command:
  - `./dialtone.sh chrome src_v1 click <selector> --host <host|all> [--url ...] [--role dev]`
- Added service-side browser log capture for actions (console/exception collection returned to CLI).
- Added Windows service host fallback for service RPC (`host`, `127.0.0.1`, `localhost`) from WSL.
- Added stable default profile path per role on service host:
  - `~/.dialtone/chrome/<role>-profile` (and `-headless` suffix for headless).
- Added service-side single-role instance cleanup and best-effort single-page-tab cleanup.

## In Progress
- Make remote actions reliable across `darkmac`, `gold`, `legion` with service-only control.
- Make `click` robust on all hosts and return actionable per-host errors + logs.
- Ensure `open` and `click` always reuse the same existing tab when present.

## Current Failures / Blockers
- `click form-submit-button --host all` currently fails:
  - `darkmac`: action timeout (`context deadline exceeded`).
  - `gold`: `Failed to open new tab - no browser is open (-32000)` from action path.
  - `legion`: response parse failure (`invalid character 'p' after top-level value`) indicating non-JSON error path from service call.
- `open --host all`:
  - `darkmac` and `gold` succeed.
  - `legion` still often falls back to legacy path with `remote windows debug port 9333 is not reachable`.
- `gold` currently has multiple `dev` instances; singleton enforcement is not yet consistently converging.
- `list --host all` still reports multiple stale/unknown instances (especially on `legion`).

## Immediate Next Steps
1. Fix `/action` handler reliability:
   - ensure it always returns JSON (even on panic/error paths).
   - harden selector handling and per-action timeout behavior.
2. Fix existing-tab attach path:
   - attach to page websocket from `/json/list` reliably.
   - avoid browser-context operations that create/open new tabs.
3. Harden singleton convergence:
   - enforce one process per role (`dev`/`test`) and one page tab after each action/open.
4. Fix legion service routing:
   - ensure service endpoint is always reachable from WSL command path.
   - stop fallback to legacy remote debug port path when service is healthy.
5. Re-verify commands:
   - `open --host all --fullscreen --url dialtone.earth`
   - `list --host all`
   - `click form-submit-button --host all --url dialtone.earth`

## Commands Being Used for Verification
- `./dialtone.sh chrome src_v1 deploy --host <host> --service --role dev`
- `./dialtone.sh chrome src_v1 open --host all --fullscreen --url dialtone.earth`
- `./dialtone.sh chrome src_v1 list --host all`
- `./dialtone.sh chrome src_v1 click form-submit-button --host all --url dialtone.earth`
