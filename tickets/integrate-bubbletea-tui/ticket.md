# Branch: integrate-bubbletea-tui
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start integrate-bubbletea-tui`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket integrate-bubbletea-tui`
- status: todo

## SUBTASK: <subtask-title>
- name: <subtask-name> (only lowercase and dashes)
- description: <description>
- test-description: <test-description>
- test-command: <test-command>
- status: todo | processing | done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done integrate-bubbletea-tui`
- status: todo

