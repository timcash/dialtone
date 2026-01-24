# Branch: install-cli-testing
# Tags: testing, cli, install, plugin

# Goal
The goal of this ticket is to implement a comprehensive testing suite for the `install` plugin. This includes creating a test scaffold, verifying CLI help output, ensuring local dependencies are installed correctly in a test environment, and verifying the idempotency of the installation process. This will ensure robust dependency management and prevent regressions across different platforms (Linux/WSL, macOS).

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start install-cli-testing`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh ticket test install-cli-testing`
- status: done

## SUBTASK: Create Install Plugin Test Scaffold
- name: create-install-test-scaffold
- description: Create `src/plugins/install/test/install_test.go` with `setupTestEnv` helper that sets `DIALTONE_ENV` to a temp directory and returns a cleanup function to avoid modifying the host environment.
- test-description: Verify the test file exists and compiles.
- test-command: `dialtone.sh plugin test install`
- status: done

## SUBTASK: Implement Install Output Test
- name: implement-install-output-test
- description: Add `TestInstallHelp` to `install_test.go` to verify `dialtone install --help` prints usage information without error.
- test-description: Run the specific test case.
- test-command: `dialtone.sh plugin test install --run TestInstallHelp`
- status: done

## SUBTASK: Implement Local Install Test
- name: implement-local-install-test
- description: Add `TestLocalInstall` to `install_test.go`. This test should use `RunInstall` with appropriate flags (e.g. `--linux-wsl` or auto-detect) to install dependencies into the temp environment, then verify key binaries (go, node, zig) exist.
- test-description: Run the specific test case.
- test-command: `dialtone.sh plugin test install --run TestLocalInstall`
- status: done

## SUBTASK: Implement Install Idempotency Test
- name: implement-install-idempotency-test
- description: Add `TestInstallIdempotency` to `install_test.go`. Run the installation twice in the same temp environment and assert it succeeds both times, and that files are still present.
- test-description: Run the specific test case.
- test-command: `dialtone.sh plugin test install --run TestInstallIdempotency`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the finial subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done install-cli-testing`
- status: todo

