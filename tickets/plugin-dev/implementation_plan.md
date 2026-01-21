# Implementation Plan - Plugin CLI Command

Refactor logic from `ticket` command to a new `plugin` command to support dedicated plugin management.

## User Review Required
> [!IMPORTANT]
> The `--plugin` flag will be removed from `ticket start`. Users must use `dialtone-dev plugin create` separately.

## Proposed Changes

### Plugin Infrastructure

#### [New] [plugin.go](file:///home/user/dialtone/src/plugins/plugin/cli/plugin.go)
- Implement `RunPlugin(args []string)`
- Implement `create(args []string)` subcommand
- Duplicate scaffolding logic from `ticket.go` (creating directories, README, test templates)

### Core CLI

#### [Modify] [dev.go](file:///home/user/dialtone/src/dev.go)
- Add `plugin_cli` import
- Add `case "plugin": plugin_cli.RunPlugin(args)` to switch statement
- Add `plugin` to `printDevUsage`

### Ticket Command Refactor

#### [Modify] [ticket.go](file:///home/user/dialtone/src/plugins/ticket/cli/ticket.go)
- Remove `RunStart` logic for `--plugin` flag
- Remove `createTestTemplates` if it's no longer used by ticket command (ticket use it for ticket tests, so keep it, but remove the branch for plugin templates if any).

## Verification Plan

### Automated Tests
- Implement and run `tickets/plugin-dev/test/e2e_test.go`
- Verify `dialtone-dev plugin create` works.

### Manual Verification
- Run `./dialtone.sh plugin create test-plugin`
- Verify `src/plugins/test-plugin` exists.
