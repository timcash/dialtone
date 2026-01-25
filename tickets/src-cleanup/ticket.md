# Branch: src-cleanup
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start src-cleanup`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket src-cleanup`
- status: done

## SUBTASK: Move and Refactor Config
- name: move-config
- description: Move `src/config.go` to `src/core/config/config.go`. Update package to `config` and fix imports.
- test-description: Verify dialtone builds
- test-command: `./dialtone.sh build --full`
- status: done

## SUBTASK: Move Web Assets
- name: move-web
- description: Move `src/web` directory to `src/core/web`. Update embedding in `dialtone.go` and path in `plugins/ui`.
- test-description: Verify web build and ui plugin commands
- test-command: `./dialtone.sh ui build`
- status: done

## SUBTASK: Move Remote Ops
- name: move-remote
- description: Move `src/remote.go` to `src/plugins/deploy/cli/remote_ops.go`. Refactor to use `cli` package.
- test-description: Verify dialtone builds and deploy plugin compiles (if applicable)
- test-command: `./dialtone.sh plugin build deploy`
- status: done

## SUBTASK: Delete Legacy Logger
- name: delete-logger
- description: Delete `src/logger.go`. Update all `dialtone.Log*` calls to `logger.Log*` (importing `src/core/logger`).
- test-description: Verify dialtone builds and logs still work
- test-command: `./dialtone.sh build --full && ./dialtone.sh logs --help`
- status: done

## SUBTASK: Move Provision to VPN Plugin
- name: move-provision
- description: Create `vpn` plugin. Move `src/provision.go` to `src/plugins/vpn/cli/vpn.go`. Update `dialtone-dev` to use the plugin.
- test-description: Verify dialtone builds and provision command triggers (even if mocked)
- test-command: `./dialtone.sh logs --help && ./dialtone.sh vpn --help`
- status: done

## SUBTASK: Documentation Verification
- name: verify-docs
- description: Verify all commands in `docs/cli.md` work as expected. Ensure ticket workflow in `docs/workflows/workflow-ticket.md` is followed.
- test-description: Run help commands for all CLI entries and verify no errors.
- test-command: `./dialtone.sh help`
- status: done

## SUBTASK: Final Verification
- name: verify-cli
- description: Final end-to-end check of CLI functionality.
- test-description: Verify key flows work (build, logs, vpn).
- test-command: `./dialtone.sh build --full`
- status: done

## SUBTASK: Debug Web UI
- name: debug-web-ui
- description: Fix Web UI embedding in `src/dialtone.go`. The embed directive and `fs.Sub` path must match the new `src/core/web` location.
- test-description: Deploy and run diagnostic.
- test-command: `./dialtone.sh deploy && ./dialtone.sh diagnostic`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- status: done

