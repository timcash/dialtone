# Plan: remote-deploy-pi

## Goal
Build, deploy, and start the `dialtone` binary on a remote Raspberry Pi (192.168.4.36), ensuring the web interface is accessible via Tailscale.

## Tests
- [x] test_sync_code: Verify `sync-code` works to the remote robot
- [x] test_remote_build: Verify `build --remote` succeeds for ARM/Pi
- [x] test_remote_deploy: Verify `deploy` sends the binary to 192.168.4.36
- [x] test_remote_start: Verify `dialtone start` command on remote robot
- [x] test_web_access: Verify Tailscale domain access to the UI (Drone UI)
- [x] test_nats_ui: Verify NATS messaging via Drone UI

## Notes
- Target IP: 192.168.4.36
- User: `tim`, Password: `password` (used `.env` for storage)
- Fixed structure tickets: `dialtone-earth` vs `src/web`
- Fixed flag parsing in `build --remote`
- Cleaned remote `src` to avoid stale `manager.go` conflicts

## Blocking Tickets
- None

## Progress Log
- 2026-01-18: Created plan file and defined goals for PI deployment
- 2026-01-18: Successfully completed build, sync, and deployment. Service is running on Tailscale.
