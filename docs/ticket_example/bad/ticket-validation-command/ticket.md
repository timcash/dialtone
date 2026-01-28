# Branch: ticket-validation-command
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket
- name: ticket-start
- description: start working on the ticket
- test-description: run `dialtone.sh ticket start ticket-validation-command`
- test-command: `./dialtone.sh ticket start ticket-validation-command`
- status: done

## SUBTASK: implement validate command
- name: implement-validate-command
- description: Implement a `ticket validate <ticket-name>` command in `src/plugins/ticket/cli/validate.go`. It should parse the ticket.md file and ensure it follows the format constraints (one ## SUBTASK per task, required fields, etc).
- test-description: Run `dialtone ticket validate ticket-validation-command` and verify it passes for a valid ticket and fails for an invalid one.
- test-command: `./dialtone.sh build && ./dialtone ticket validate ticket-validation-command`
- status: done

## SUBTASK: ticket done
- name: ticket-done
- description: finish the ticket
- test-description: run `dialtone.sh ticket done ticket-validation-command`
- test-command: `./dialtone.sh ticket done ticket-validation-command`
- status: done
