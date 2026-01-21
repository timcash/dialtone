# Branch: github-plugin
# Task: Move Pull Request Wrapper to Github Plugin

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if needed

## Goals
1. Use tests files in `tickets/github-plugin/test/` to drive all work.
2. Create `dialtone-dev github` command and plugin structure.
3. Move `dialtone-dev pull-request` logic into the new plugin.
4. Ensure `dialtone-dev pull-request` delegates to the new plugin.
5. Address Vercel deployment failures in PRs by adding status checks or fixes.

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.
2. DO NOT rewrite the entire `gh` CLI wrapper logic if not necessary, just move it.

## Test
1. all ticket tests are at `tickets/github-plugin/test/`
2. all plugin tests are run with `./dialtone.sh plugin test github`
3. all core tests are run with `./dialtone.sh test --core`
4. all tests are run with `./dialtone.sh test`

## Subtask: Research
- description: Analyze `src/dev.go` to understand `runPullRequest` logic and dependencies.
- status: done
- description: Investigate why Vercel deployments are failing in PRs and how to detect/fix this.
- status: todo

## Subtask: Scaffold & Test Setup
- description: Run `./dialtone.sh plugin create github` to generate plugin structure.
- status: todo
- description: [NEW] `tickets/github-plugin/test/e2e_test.go`: Implement test to verify `dialtone-dev github` command exists and `pull-request` subcommand works (mocking `gh` if needed/possible).
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/github/cli/github.go`: Implement `RunGithub` entry point.
- status: todo
- description: [MODIFY] `src/plugins/github/cli/github.go`: Move `runPullRequest` logic from `src/dev.go` to here as `pull-request` subcommand. Ensure it supports all flags (`--title`, `--body`, `--draft`, `--ready`, `--view`) and plan file integration.
- status: todo
- description: [MODIFY] `src/dev.go`: Import `dialtone/cli/src/plugins/github/cli`, add `github` command case, and change `pull-request` case to delegate to `plugin_cli.RunGithub` with `pull-request` arg.
- status: todo
- description: [NEW/MODIFY] `src/plugins/github/cli/github.go`: Implement logic to check/wait for Vercel deployment status (e.g., `dialtone-dev github check-deploy`).
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh ticket test github-plugin`
- status: todo
- description: Run test: `./dialtone.sh plugin test github`
- status: todo
- description: Run test: `./dialtone.sh test`
- status: todo

## Collaborative Notes
- The current `pull-request` command in `src/dev.go` handles various flags (`--title`, `--body`, `--draft`, `--ready`, `--view`) and interacts with `gh` CLI.
- We need to ensure all these flags are preserved in the new `github` plugin.
- The `plan` file integration (reading body from plan file) must also be preserved.
- Usage of `gh` CLI implies we might need to mock it in tests or rely on manual verification for the actual GitHub interaction parts, but we can verify the command parsing and delegation.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
