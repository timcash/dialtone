# Plan: remote-deploy-pi

## Goal
Build, deploy, and start the `dialtone` binary on a remote Raspberry Pi (192.168.4.36), ensuring the web interface is accessible via Tailscale.

## Tests
- [ ] test_sync_code: Verify `sync-code` works to the remote robot
- [ ] test_remote_build: Verify `build --remote` succeeds for ARM/Pi
- [ ] test_remote_deploy: Verify `deploy` sends the binary to 192.168.4.36
- [ ] test_remote_start: Verify `dialtone start` command on remote robot
- [ ] test_web_access: Verify Tailscale domain access to the UI

## Notes
- Target IP: 192.168.4.36
- Using `dialtone-dev` CLI as guided by `agent.md`
- Building for the robot may be out of date, need to verify and possibly fix

## Blocking Issues
- None

## Progress Log
- 2026-01-18: Created plan file and defined goals for PI deployment
