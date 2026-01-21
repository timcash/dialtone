# Branch: plugin-dev
# Task: Create Plugin Command

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if needed

## Goals
1. Use tests files in `tickets/plugin-dev/test/` to drive all work.
2. Create `dialtone-dev plugin` command.
3. Move `ticket start --plugin` functionality to `plugin create` subcommand.
4. Ensure `plugin create` generates the same folder structure and test templates as the current `--plugin` flag.

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.

## Test
1. all ticket tests are at `tickets/plugin-dev/test/`
2. all plugin tests are run with `./dialtone.sh plugin test plugin`
3. all core tests are run with `./dialtone.sh test --core`
4. all tests are run with `./dialtone.sh test`

## Subtask: Research
- description: Analyze `src/plugins/ticket/cli/ticket.go` to understand current `RunStart` plugin scaffolding logic.
- status: done

## Subtask: Scaffold & Test Setup
- description: Verify `tickets/plugin-dev/test/` exists (created by start command).
- description: [NEW] `tickets/plugin-dev/test/e2e_test.go`: Implement test to verify `dialtone-dev plugin create test-plugin` creates correct directory structure (`src/plugins/test-plugin/{app,cli,test}`).
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/plugin/cli/plugin.go`: Implement `RunPlugin` entry point and `create` subcommand logic.
- description: [MODIFY] `src/dev.go`: Import `dialtone/cli/src/plugins/plugin/cli` and delegate `plugin` command to it.
- description: [MODIFY] `src/plugins/ticket/cli/ticket.go`: Remove `--plugin` scaffolding logic from `RunStart`.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test` (which triggers the ticket tests).
- status: todo

## Collaborative Notes
- We are refactoring the CLI to be more modular.
- The `plugin` command will host all plugin-related utilities, starting with `create`.
- Legacy `ticket start --plugin` should be removed to enforce new usage.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
