# Branch: refine-ticket-add-docs
# Tags: <tags>

# Goal
Update `src/plugins/ticket/README.md` to explicitly state that `ticket add` does not switch branches, making it suitable for agents to log side-tasks or bugs without losing current context.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start refine-ticket-add-docs`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket refine-ticket-add-docs`
- status: done

## SUBTASK: verify ticket add behavior
- name: verify-behavior
- description: verify that `ticket add` only scaffolds files and does not perform git operations like checkout.
- test-description: run `ticket add test-add-behavior` and check `git branch --show-current` remains unchanged.
- test-command: `bash -c "./dialtone.sh ticket add test-ticket-add-behavior && [ \"$(git branch --show-current)\" = \"refine-ticket-add-docs\" ] && rm -rf tickets/test-ticket-add-behavior"`
- status: done

## SUBTASK: update readme with agentic use case
- name: update-readme
- description: Update `src/plugins/ticket/README.md` section for `ticket add`. explicitly mention it does not switch branches and is useful for agents to "log bugs" or "create tasks" for later without context switching.
- test-description: grep for the new explanation
- test-command: `grep "does not switch branches" src/plugins/ticket/README.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done refine-ticket-add-docs`
- status: todo

