# Plan: www-integration

## Goal
Integrate the public dialtone.earth webpage into the main repository and provide management commands via the `dialtone-dev www` CLI tool.

## Tests
- [x] test_www_subcommand: Verify `go run dialtone-dev.go www` shows usage with publish, logs, and domain
- [x] test_repo_integration: Verify `dialtone-earth/` folder contains the integrated webpage code
- [x] test_vercel_publish: Verify `go run dialtone-dev.go www publish` calls vercel CLI correctly
- [x] test_vercel_logs: Verify `go run dialtone-dev.go www logs` calls vercel CLI correctly
- [x] test_vercel_domain: Verify `go run dialtone-dev.go www domain` calls vercel CLI to manage dialtone.earth

## Notes
- `dialtone-earth` code is currently at https://github.com/timcash/dialtone-earth-0o
- The `www` command will be a wrapper around the Vercel CLI (`vercel`)
- Domain management should point `dialtone.earth` to the `dialtone-earth/` subfolder

## Blocking Issues
- None

## Progress Log
- 2026-01-16: Created plan file, implemented commands, and verified deployment
