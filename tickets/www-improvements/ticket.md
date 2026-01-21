# Branch: www-improvements
# Task: WWW Improvements

> IMPORTANT: run `./dialtone.sh ticket start <this-file> --plugin www` to start work it will create needed files.

## Goals\
0. verify `./dialtone.sh ticket start <this-file> --plugin www` ran corretly it is new code 
1. Use tests files in `tickets/www-improvements/test/` to drive all work.
2. Migrate the existing `dialtone-earth` Next.js application into the `src/plugins/www/app` directory.
3. Refactor the `www` command logic from `src/dev.go` into a dedicated `src/plugins/www/cli` package.

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.
2. DO NOT manually deploy to Vercel during development; use dry-run or mock tests.
3. DO NOT change the visual design or functionality of the `dialtone-earth` app itself, only its location and CLI wrapper.

## Test
1. all ticket tests are at `tickets/www-improvements/test/`
2. all plugin tests are run with `./dialtone.sh plugin test www` (not built yet)
3. all core tests are run with `./dialtone.sh test --core` (not built yet)
4. all tests are run with `./dialtone.sh test`

## Subtask: Research
- description: Analyze `src/dev.go` `runWww` function and `dialtone-earth` structure.
- status: done

## Subtask: Scaffold & Test Setup
- description: Verify `tickets/www-improvements/test/` exists (created by start command).
- description: [NEW] `tickets/www-improvements/test/e2e_test.go`: Implement test to verify `dialtone.sh www dev` starts a server (check localhost:3000).
- description: [NEW] `tickets/www-improvements/test/integration_test.go`: Implement test to verify `dialtone.sh www build` creates expected artifacts.
- status: done

## Subtask: App Migration
- description: Move all contents from `dialtone-earth/` to `src/plugins/www/app/`.
- status: done

## Subtask: CLI Refactor
- description: [NEW] `src/plugins/www/cli/www.go`: Implement `RunWww` function handling `dev`, `build`, `publish`, `logs`, `domain`, `login`.
- description: [MODIFY] `src/dev.go`: Import `dialtone/cli/src/plugins/www/cli` and delegate `www` command to it. Remove `runWww` implementation.
- status: done

## Subtask: Verification
- description: Run test: `./dialtone.sh test` (which triggers the ticket tests).
- status: done

## Development Cycle
1. Run `./dialtone.sh ticket start www_improvements` to change the git branch (already done if you are here).
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Update `ticket.md` to reflect subtasks completed and those remaining.
5. Update `task.md` to reflect scratchpad notes and research.
6. Use `git add` to update git and ensure `.gitignore` is correct.
7. Update `docs/vendor/<vendor_name>.md` to remeber any important vendor specific information like summaries of thier docs, hardware specs, links, etc.
8. Make a commit with `git commit -m "<message>"`. so you can revert to working tests if needed.

## Development Stages
1. **Ticket**: The first step of any change. Ideal for adding new code that can patch `core` or `plugin` code without changing it directly.
2. **Plugin**: The second step of integrating new code into specific feature areas.
3. **Core**: Core code is reserved for features dealing with networking and deployment (dialtone/dialtone-dev). It is the minimal code required to bootstrap the system.

## Collaborative Notes
- We are moving `dialtone-earth` to `src/plugins/www/app`.
- We must ensure that the `e2e_test.go` in the ticket folder properly invokes the CLI and checks the output/side-effects.
- `tickets/www-improvements/test/` will be the primary driver for verification.

---
# Ticket folder layout:
1. `ticket.md` - this file
2. `task.md` - a scratchpad for tracking progress that is not mentioned in `ticket.md`
3. `code/` - all code developed for this change.
4. `test/` - all tests that run to verify this ticket.

---
Template version: 4.1.