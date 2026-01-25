# Branch: new-ticket-subtask-subcommand
# Tags: [cli, dev-ux, ticket]

# Goal
Implement the `dialtone.sh ticket subtask` suite of commands to manage ticket granularity and workflow. This allows developers to break down tickets into manageable, testable steps and track progress directly from the CLI.
**Note:** The `ticket` plugin already exists in `src/plugins/ticket`. This work extends the existing plugin.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start <ticket-name>`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket <ticket-name>`
- status: todo

## SUBTASK: Implement subtask list command
- name: implement-subtask-list-command
- description: Create the `dialtone.sh ticket subtask list <ticket-name>` command. This command should parse the `tickets/<ticket-name>/ticket.md` file and list all subtasks found within.
- test-description: Create a dummy ticket with known subtasks and assert the output of the list command matches expected output.
- test-command: `dialtone.sh test ticket new-ticket-subtask-subcommand`
- status: done

## SUBTASK: Implement subtask next command
- name: implement-subtask-next-command
- description: Create the `dialtone.sh ticket subtask next <ticket-name>` command. It should find the first subtask in `ticket.md` that is not 'done' and print it.
- test-description: Create a dummy ticket with mixed status subtasks and assert `next` returns the first non-done one.
- test-command: `dialtone.sh test ticket new-ticket-subtask-subcommand`
- status: done

## SUBTASK: Implement subtask test command
- name: implement-subtask-test-command
- description: Create the `dialtone.sh ticket subtask test <ticket-name> <subtask-name>` command. It should run the `test-command` specified in the subtask.
- test-description: Create a dummy ticket with a subtask having a simple echo test command. Verify it executes.
- test-command: `dialtone.sh test ticket new-ticket-subtask-subcommand`
- status: done

## SUBTASK: Implement subtask done command
- name: implement-subtask-done-command
- description: Create the `dialtone.sh ticket subtask done <ticket-name> <subtask-name>` command. It should update the status of the specified subtask to 'done' in the `ticket.md` file.
- test-description: Run the done command on a dummy ticket and verify the file content is updated to 'status: done'.
- test-command: `dialtone.sh test ticket new-ticket-subtask-subcommand`
- status: done

## SUBTASK: Wire up main subtask command
- name: wire-up-main-subtask-command
- description: Ensure `dialtone.sh ticket subtask` routes to the correct subcommand (list, next, test, done) in the `ticket` plugin.
- test-description: Verify help output or error when no subcommand is provided.
- test-command: `dialtone.sh test ticket new-ticket-subtask-subcommand`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the finial subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done <ticket-name>`
- status: todo
