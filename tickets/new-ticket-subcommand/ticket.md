# Branch: new-ticket-subcommand
# Tags: [cli, dev-ux, ticket]

#CAPABILITY: DevTools

## #SUBTASK: Research CLI Infrastructure
- description: Research `src/dev.go` and `src/plugins/ticket/cli/ticket.go` to identify where to hook the new `ticket new` subcommand.
- test: Create a unit test `TestCLIRegistration` in `tickets/new-ticket-subcommand/test/unit_test.go` that verifies the `RunTicket` function call path.
- status: done

## #SUBTASK: Implement Ticket Template Copy Logic
- description: Create `RunNew` in `src/plugins/ticket/cli/ticket.go` to copy `tickets/template-ticket/ticket.md` to `tickets/<name>/ticket.md` and replace placeholder branch names.
- test: Create an integration test `TestTemplateCopy` in `tickets/new-ticket-subcommand/test/integration_test.go` that verifies file creation and content replacement.
- status: done

## #SUBTASK: Register `ticket new` Subcommand
- description: Update the sub-command dispatcher in `src/plugins/ticket/cli/ticket.go` and the usage help in `src/dev.go` to include the `new` command.
- test: Run `./dialtone.sh ticket` and verify `new <name>` is listed in the help output.
- status: done

## #SUBTASK: Final Verification
- description: Verify the complete end-to-end flow of creating a new ticket using the CLI.
- test: Run `./dialtone.sh ticket new e2e-test-ticket && grep "# Branch: e2e-test-ticket" tickets/e2e-test-ticket/ticket.md`.
- status: done