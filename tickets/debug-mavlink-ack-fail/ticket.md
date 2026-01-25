# Branch: debug-mavlink-ack-fail
# Tags: bugfix, mavlink

# Goal
Fix the incorrect "ACK: FAIL" reports in the frontend by ensuring the MAVLink result code is correctly serialized as an integer.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start debug-mavlink-ack-fail`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket debug-mavlink-ack-fail`
- status: done

## SUBTASK: investigate ack failure
- name: investigate-ack
- description: Locate the code responsible for MAVLink ACK serialization and confirm why it is failing validation in the frontend.
- test-description: Manual Code Review / Log Verification
- test-command: `grep -r "COMMAND_ACK" src/`
- status: done

## SUBTASK: fix result type casting
- name: fix-type-casting
- description: Modify `src/dialtone.go` to explicitly cast `msg.Result` to `int` before JSON marshaling.
- test-description: Run dialtone tests to ensure no regression.
- test-command: `dialtone.sh test`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done debug-mavlink-ack-fail`
- status: todo
