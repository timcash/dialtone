# Robot src_v2 Diagnostic Checklist

This is the required check list for `./dialtone.sh robot src_v2 diagnostic --host <host>`.

## 1) Local Artifact Preconditions
1. `<repo_root>/bin/dialtone_autoswap_v1` exists and is executable.
2. `<repo_root>/bin/dialtone_robot_v2` exists and is executable.
3. `<repo_root>/bin/dialtone_camera_v1` exists and is executable.
4. `<repo_root>/bin/dialtone_mavlink_v1` exists and is executable.
5. `<repo_root>/bin/dialtone_repl_v1` exists and is executable.
6. `<repo_root>/src/plugins/robot/src_v2/ui/dist/index.html` exists.

## 2) Remote Runtime Artifact Checks
1. Remote autoswap binary exists at `<remote_repo>/bin/dialtone_autoswap_v1`.
2. Remote robot/camera/mavlink/repl binaries exist in `<remote_repo>/bin`.
3. Remote `ui/dist/index.html` exists at `src/plugins/robot/src_v2/ui/dist/index.html`.
4. Target manifest file exists (absolute or repo-relative path accepted).

## 3) Autoswap Service Checks
1. `systemctl --user is-active dialtone_autoswap.service` returns `active`.
2. `systemctl --user show dialtone_autoswap.service --property=ExecStart` includes:
- `dialtone_autoswap_v1`
- the expected manifest path
3. `dialtone_autoswap_v1 service --mode list ...` succeeds remotely.
4. Autoswap state files exist and are readable:
- `~/.dialtone/autoswap/state/supervisor.json`
- `~/.dialtone/autoswap/state/runtime.json`
5. Runtime state `manifest_path` matches expected active manifest.

## 4) Autoswap Managed Process Checks
1. Runtime state includes all required managed process entries:
- `robot`
- `camera`
- `mavlink`
- `repl`
2. Each required entry reports:
- `status = running`
- `pid > 0`
3. Remote process list includes:
- `dialtone_robot_v2`
- `dialtone_camera_v1`
- `dialtone_mavlink_v1`
- `dialtone_repl_v1`

## 5) Endpoint and Integration Checks
1. `GET http://127.0.0.1:18086/health` returns `ok`.
2. `GET http://127.0.0.1:18086/api/init` includes `/natsws`.
3. `GET http://127.0.0.1:18086/api/integration-health` includes:
- `"camera":{"status":"configured"}`
- `"mavlink":{"status":"ok"}`
4. `GET http://127.0.0.1:18086/stream` returns HTTP `200`.

## 6) UI Chromedp Checks
1. Open UI URL (default `http://<host>:18086`, or explicit `--ui-url`).
2. Verify hero section is active:
- `aria-label="Hero Section"`
- `data-active="true"`
3. Verify menu open + section navigation works for:
- Docs
- Telemetry
- Three
- Terminal
- Camera
- Settings
4. For each section, verify destination aria label exists and `data-active="true"`.
5. Docs section must include version token:
- `ROBOT_UI_DOCS_VERSION: robot-src_v2-docs-v4`
6. Telemetry section check includes `aria-label="Robot Table"`.

## 7) Optional Flags for Diagnostic
1. `--manifest` to validate a non-default manifest path.
2. `--remote-repo` when repo is not at `<home>/dialtone`.
3. `--ui-url` for public/tunneled endpoint checks.
4. `--browser-node` to force mesh-host browser for chromedp.
5. `--skip-ui` to run service/process/endpoints checks only.
