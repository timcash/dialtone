# Branch: remove-geminikey-usage
# Tags: <tags>

# Goal
Remove all `geminiKey` usage from the AI plugin and ensure it strictly uses `GOOGLE_API_KEY`.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start remove-geminikey-usage`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket remove-geminikey-usage`
- status: done

## SUBTASK: verify removal of geminiKey usage
- name: verify-removal
- description: Ensure no occurrences of `geminiKey` remain in the src/plugins/ai/cli directory.
- test-description: Run a grep search to verify no occurrences exist.
- test-command: `dialtone.sh test ticket remove-geminikey-usage --subtask verify-removal`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done remove-geminikey-usage`
- status: todo

