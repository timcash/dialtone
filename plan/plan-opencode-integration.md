# Plan: opencode-integration

## Goal
Integrate the `opencode` AI assistant server into the Dialtone CLI tools (`dialtone` and `dialtone-dev`).

## Tests
- [x] test_dev_cli_opencode: Verify `dialtone-dev opencode` commands (start, stop, status, ui)
- [x] test_dialtone_opencode_flag: Verify `dialtone start --opencode` starts the assistant
- [x] test_tailscale_proxy: Verify opencode port 3000 is proxied over Tailscale

## Notes
- `opencode` is an AI code assistant server.
- Integrated into `dialtone-dev` for manual management.
- Integrated into `dialtone` for automatic deployment and secure access via Tailscale.

## Blocking Issues
- None

## Progress Log
- 2026-01-19: Created plan file
- 2026-01-19: Implemented integration in `src/dev.go` and `src/dialtone.go`
- 2026-01-19: Fixed missing imports and port initialization
- 2026-01-19: Verified with E2E tests

Closes #32
