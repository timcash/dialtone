# Branch: improve-plugin-install-paths
# Tags: p0, install-system

# Goal
Refactor the plugin installation system to support distinct `dev` and `production` paths. Move all plugin-specific installation and build logic from the core into `install.go` and `build.go` within each plugin's directory.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start improve-plugin-install-paths`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket improve-plugin-install-paths`
- status: done

## SUBTASK: Research and design dev/prod install paths
- name: design-install-paths
- description: Audit current `install` plugin logic and define the interface for `install.go` and `build.go` to support environments like dialtone-env vs system global.
- test-description: Create a design doc or RFC in the collaborative notes section.
- test-command: `ls tickets/improve-plugin-install-paths/design.md`
- status: todo

## SUBTASK: Migrate GO plugin install logic
- name: migrate-go-install
- description: Move Go environment setup logic from `src/plugins/go/cli/go.go` (or wherever it lives) to `src/plugins/go/install.go`.
- test-description: Verify go plugin installs correctly in a clean DIALTONE_ENV.
- test-command: `DIALTONE_ENV=/tmp/dt-test ./dialtone.sh plugin install go`
- status: todo

## SUBTASK: Implement Build step for AI plugin
- name: build-ai-plugin
- description: Implement `src/plugins/ai/build.go` to handle any necessary binary compilation or dependency bundling.
- test-description: Verify the plugin builds successfully and the binary is placed in the correct bin path.
- test-command: `./dialtone.sh plugin build ai`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done improve-plugin-install-paths`
- status: todo

## Collaborative Notes
- Current install logic is centralized in `src/plugins/install/cli/install.go`.
- Need to ensure `dialtone-env` isolation is maintained.
- Production paths should likely point to `/usr/local/bin` or similar on target hardware.
