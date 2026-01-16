# Plan: dialtone-dev-cli

## Goal
Create the `dialtone-dev` CLI tool for development workflow automation. This tool helps manage feature branches, plans, tests, and pull requests as described in `agent.md`.

## Tests
- [x] test_plan_create: Verify `dialtone-dev plan <name>` creates a new plan file from template
- [x] test_plan_list: Verify `dialtone-dev plan` lists existing plan files
- [x] test_plan_template: Verify plan template includes all required sections (Goal, Tests, Notes, Progress Log)
- [x] test_branch_create: Verify `dialtone-dev branch <name>` creates a new feature branch
- [x] test_branch_checkout: Verify `dialtone-dev branch <name>` checks out existing branch if it exists
- [x] test_test_run_all: Verify `dialtone-dev test` runs all tests in test/ directory
- [x] test_test_run_feature: Verify `dialtone-dev test <name>` runs tests in test/<name>/
- [x] test_pull_request: Verify `dialtone-dev pull-request` creates or updates a PR

## Commands to Implement

### 1. `dialtone-dev plan <name>`
- If `plan/plan-<name>.md` exists, print its contents
- If not, create a new plan file from template
- Template includes: Goal, Tests (empty checklist), Notes, Blocking Issues, Progress Log

### 2. `dialtone-dev plan` (no args)
- List all existing plan files in `plan/` directory
- Show completion status (count of checked/total items)

### 3. `dialtone-dev branch <name>`
- Check if branch exists: `git branch --list <name>`
- If exists, checkout: `git checkout <name>`
- If not, create: `git checkout -b <name>`

### 4. `dialtone-dev test` / `dialtone-dev test <name>`
- Run `go test -v ./test/...` or `go test -v ./test/<name>/...`

### 5. `dialtone-dev pull-request`
- Use `gh pr create` or `gh pr edit` via GitHub CLI

## Notes
- Entry point: `dialtone-dev.go` in root directory
- Implementation: `src/dev.go` 
- Binary: `bin/dialtone-dev`
- Follow same pattern as main `dialtone` CLI

## Progress Log
- 2026-01-16: Created plan file and feature branch
- 2026-01-16: Implemented all commands (plan, branch, test, pull-request)
- 2026-01-16: Added test suite with 10 unit tests (test/dialtone-dev-cli/unit_test.go)