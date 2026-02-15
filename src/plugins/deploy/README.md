# Plugin: deploy

Deploys the Dialtone binary to a remote robot over SSH. The command auto-detects
the remote CPU architecture and cross-compiles locally before uploading and
starting the service on the target.

## Deploy and run diagnostics
1. Export required env vars (or pass flags):
   - `ROBOT_HOST`, `ROBOT_USER`, `ROBOT_PASSWORD`
   - `TS_AUTHKEY`, `DIALTONE_HOSTNAME`
2. Deploy:
   - `dialtone deploy --host user@robot --user robot --pass "$ROBOT_PASSWORD"`
3. Run diagnostics against the same host:
   - `dialtone diagnostic --host user@robot --user robot --pass "$ROBOT_PASSWORD"`

## How deploy works
- Connects to the remote host over SSH using `ROBOT_HOST`, `ROBOT_USER`,
  `ROBOT_PASSWORD` (or `--host/--user/--pass` flags).
- Runs `uname -m` remotely to detect architecture.
- Cross-compiles locally via `dialtone build --local --linux-arm64`,
  `--linux-arm`, or `--linux-amd64` (depending on the remote architecture).
- Uploads the binary to `REMOTE_DIR_DEPLOY` (defaults to `~/dialtone_deploy`).
- Restarts the service remotely with `TS_AUTHKEY`, `DIALTONE_HOSTNAME`, and
  optional `MAVLINK_ENDPOINT`.

## OS-specific notes
### WSL / Linux
- Cross-compilation prefers Podman when available. On WSL you can install it
  directly (`sudo apt install -y podman`) and it runs natively.
- No VM/daemon is required; `podman` just needs to be on the PATH.

### macOS
- Cross-compilation also prefers Podman, but Podman runs in a VM on macOS.
- Ensure the Podman machine is installed and running before deploy
  (for example, via Podman Desktop or `podman machine start`).
- If Podman is not available, the build falls back to local tooling (Zig in
  `DIALTONE_ENV`) for cross-compilation.
