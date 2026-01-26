# Branch: update-plugin-docs
# Tags: docs

# Goal
Update README documentation for ticket and github plugins to move descriptive text into bash code blocks.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start update-plugin-docs`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `./dialtone.sh test ticket update-plugin-docs`
- status: done

## SUBTASK: Update Ticket Plugin Documentation
- name: update-ticket-docs
- description: Move descriptive text for ticket commands into bash code blocks in src/plugins/ticket/README.md
- test-description: Verify the file content matches the desired format
- test-command: `cat src/plugins/ticket/README.md`
- status: done

## SUBTASK: Update GitHub Plugin Documentation
- name: update-github-docs
- description: Move descriptive text for github commands into bash code blocks in src/plugins/github/README.md
- test-description: Verify the file content matches the desired format
- test-command: `cat src/plugins/github/README.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done update-plugin-docs`
- status: todo
