# Branch: ticket-start-final-check
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start ticket-start-final-check`
- test-description: run `./dialtone.sh plugin test <plugin-name>` to verify the ticket is valid
- test-command: `./dialtone.sh plugin test <plugin-name>`
- status: todo

## SUBTASK: <subtask-title>
- name: <subtask-name> (only lowercase and dashes)
- description: <description>
- test-description: <test-description>
- test-command: <test-command>
- status: todo | processing | done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket. if it completes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done ticket-start-final-check`
- status: todo

