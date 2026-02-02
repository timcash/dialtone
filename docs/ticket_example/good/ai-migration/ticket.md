# Branch: ai-migration
# Tags: <tags>

# Goal
Migrate `autocode` (developer loop) and `opencode` (AI assistant server) from `src/dev.go` and `src/dialtone.go` into a new `ai` plugin and CLI tool as requested by the user.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start ai-migration`
- test-description: run `./dialtone.sh plugin test <plugin-name>` to verify the ticket is valid
- test-command: `./dialtone.sh plugin test <plugin-name>`
- status: done

## SUBTASK: ai-build
- name: ai-build
- description: Implement `dialtone.sh ai build` in the plugin.
- test-description: Verify `./dialtone.sh ai build` works.
- test-command: `./dialtone.sh ai build`
- status: done

## SUBTASK: integrate-build
- name: integrate-build
- description: Integrate `ai build` into the main `dialtone build` command.
- test-description: Verify `dialtone build --full` calls AI build.
- test-command: `./dialtone.sh build --full`
- status: done

## SUBTASK: ai-docs
- name: ai-docs
- description: Create a comprehensive `README.md` in `src/plugins/ai/` explaining how the plugin starts, its developer loop, and server components.
- test-description: Verify file exists and has content.
- test-command: `cat src/plugins/ai/README.md`
- status: done

## SUBTASK: ai-test-init
- name: ai-test-init
- description: Create the test file `src/plugins/ai/test/ai_suite.go` with standard Go test boilerplate.
- test-description: Verify file exists and is registered.
- test-command: `./dialtone.sh plugin test ai --list`
- status: done

## SUBTASK: ai-test-binary
- name: ai-test-binary
- description: Implement a test to verify the `opencode` binary is present and reachable in the shell path.
- test-description: Run the specific test case via tags.
- test-command: `dialtone-dev test tags ai binary`
- status: done

## SUBTASK: ai-test-cli-version
- name: ai-test-cli-version
- description: Implement a test that runs `opencode --version` to verify the CLI responds correctly without requiring an API key.
- test-description: Run the specific test case via tags.
- test-command: `dialtone-dev test tags ai cli`
- status: done

## SUBTASK: ai-test-delegation
- name: ai-test-delegation
- description: Test the delegation logic from `dialtone.sh ai` to the plugin by verifying help output.
- test-description: Run the CLI help command and check for AI options.
- test-command: `./dialtone.sh ai help | grep -E "opencode|developer|subagent"`
- status: done

## SUBTASK: ai-test-verify-all
- name: ai-test-verify-all
- description: Run the full plugin test suite using the standard test runner.
- test-description: Verify all AI plugin tests pass.
- test-command: `./dialtone.sh plugin test ai`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done ai-migration`
- status: done



