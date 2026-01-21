# Branch: github-plugin
# Task: Move Pull Request Wrapper to Github Plugin

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if needed, it should tell you if the plugin already exists.

## Goals
1. Use tests files in `tickets/github-plugin/test/` to drive all work.
2. Create `dialtone-dev github` command.
3. Move `dialtone-dev pull-request` logic into a new plugin with cli command `dialtone-dev github pull-request`.
4. Update `dialtone-dev pull-request` to delegate to the new plugin code.

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.

## Test
1. all ticket tests are at `tickets/github-plugin/test/`
2. all plugin tests are run with `./dialtone.sh plugin test github`
3. all core tests are run with `./dialtone.sh test --core`
4. all tests are run with `./dialtone.sh test`

## Subtask: Research
- description: Analyze `src/dev.go` to understand `runPullRequest` logic and dependencies.
- status: done

## Subtask: Scaffold & Test Setup
- description: Run `./dialtone.sh plugin create github` to generate plugin structure.
- description: [NEW] `tickets/github-plugin/test/e2e_test.go`: Implement test to verify `dialtone-dev github` command exists and `pull-request` subcommand works.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/github/cli/github.go`: Implement `RunGithub` entry point and move `runPullRequest` logic here.
- description: [MODIFY] `src/dev.go`: Import `dialtone/cli/src/plugins/github/cli`, add `github` command, and update `pull-request` command to call `github` plugin logic.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh ticket test github-plugin`
- description: Run test: `./dialtone.sh plugin test github`
- description: Run test: `./dialtone.sh test`
- status: todo

## Collaborative Notes
- The current `pull-request` command in `src/dev.go` handles various flags (`--title`, `--body`, `--draft`, `--ready`, `--view`) and interacts with `gh` CLI.
- We need to ensure all these flags are preserved in the new `github` plugin.
- The `plan` file integration (reading body from plan file) must also be preserved.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
