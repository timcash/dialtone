# Branch: ticket-install-logs
# Task: Test `dialtone.sh install` dependency setup

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if this ticket is about a plugin
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests in `tickets/install-logs/test/` to validate `dialtone.sh install`.
2. Ensure install defaults to local OS/arch and `dialtone_dependencies` next to `dialtone.sh`.
3. Validate optional install path argument and env var override for `dialtone.sh`, `go run dialtone-dev.go`, and `bin/dialtone-dev`.
4. Support installing dependencies into the repo during development via CLI option or env var, and keep the folder ignored by git.

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.
2. DO NOT change core install behavior beyond what tests require.
3. DO NOT introduce new dependency versions unless needed by tests.

## Test
1. All ticket tests are at `tickets/install-logs/test/`.
2. All core tests are run with `./dialtone.sh test --core`.
3. All tests are run with `./dialtone.sh test`.

## Logging
1. Use the `src/logger.go` package to log messages.
2. Add clear logs for required env vars and install/build paths.

## Subtask: Research
- description: Review current install flow in `dialtone.sh`, env vars in `.env.example`, and usage in `dialtone-dev.go` and `bin/dialtone-dev`.
- test: Notes captured in Collaborative Notes with current behavior and gaps.
- status: done

## Subtask: Implementation
- description: [NEW] `tickets/install-logs/test/`: Tests for default install path, optional path argument, and env var override behavior.
- test: Ticket tests cover default path, CLI path, and env var precedence.
- status: done

## Subtask: Implementation
- description: [MODIFY] `dialtone.sh`: Add CLI option + env var handling for dependency location and log which source is used.
- test: Ticket tests assert install uses CLI/ENV/default and logs the source.
- status: done

## Subtask: Implementation
- description: [MODIFY] `dialtone-dev.go` and `bin/dialtone-dev`: Resolve dependency location from env var and emit clear install/build guidance.
- test: Ticket tests assert both entrypoints resolve tools from dependency path.
- status: done

## Subtask: Implementation
- description: [MODIFY] `.env.example`: Document required env vars and defaults for dependency location.
- test: Ticket tests or snapshot confirm `.env.example` includes new env var docs.
- status: done

## Subtask: Implementation
- description: [MODIFY] `.gitignore`: Ignore repo-local dependency folder used in development.
- test: Ticket tests assert repo-local dependency path is ignored by git.
- status: done

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: done

## Subtask: Verification
- description: Run test: `./dialtone.sh test --core`
- test: Core tests pass.
- status: done

## Subtask: Verification
- description: Run ticket tests: `./dialtone.sh test --ticket tickets/install-logs`
- test: Ticket tests pass.
- status: done

## Collaborative Notes
- Default install location should be `dialtone_dependencies` in the same directory as `dialtone.sh`.
- Optional install path should be accepted by `dialtone.sh install` and stored in an env var used by `dialtone.sh`, `go run dialtone-dev.go`, and `bin/dialtone-dev`.
- During development, allow dependencies in the `dialtone` repo folder; add the folder to `.gitignore` to prevent commits.
- Allow users to specify the dependency location via a command line option or environment variable.
- All three entrypoints should resolve `go`, `zig`, `npm`, and `pixi` from the dependency folder.
- May need to clean up or extend env vars in `.env.example` and improve install/build logging in `dialtone.sh` and `dialtone-dev.go`.

## Test Notes
- Default path test: run install without args/env and assert tools resolve from `dialtone_dependencies` next to `dialtone.sh`.
- CLI option test: run install with explicit path and assert env var is set and tools resolve from that path.
- Env var test: set the env var and assert install uses it over defaults and logs the source.
- Repo-local path test: install into `./dialtone_dependencies` under repo root and ensure `.gitignore` prevents git tracking.
- Logging test: assert log output mentions required env vars and how to set them for `dialtone.sh` and `dialtone-dev.go`.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
