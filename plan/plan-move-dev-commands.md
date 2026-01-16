# Plan: move-dev-commands

## Goal
Move all development-related commands from the main `dialtone` CLI into the `dialtone-dev` CLI. This simplifies the core node binary and makes bootstrapping easier as it won't require CGO/camera dependencies for the dev CLI.

## Tests
- [ ] test_dev_install: Verify `go run dialtone-dev.go install` works and command is removed from `dialtone`
- [ ] test_dev_build: Verify `go run dialtone-dev.go build` works and command is removed from `dialtone`
- [ ] test_dev_deploy: Verify `go run dialtone-dev.go deploy` works and command is removed from `dialtone`
- [ ] test_prod_start: Verify `go run dialtone.go start` still works as the primary node command
- [ ] test_dev_help: Verify help messages reflect the updated command locations

## Notes
- Commands to move: build, full-build, deploy, install, clone, sync-code, remote-build, provision, logs, diagnostic, ssh.
- `dialtone` main will only keep `start`.
- `dialtone-dev` already handles plan, branch, test, pull-request, issue, www.

## Blocking Issues
- None

## Progress Log
- 2026-01-16: Created plan file, identified commands to move.
