# Name: ticket-ask-question
# Tags: ticket, plugin, docs

# Goal
Add `ticket ask` and `ticket log` commands that write `log.md` entries alongside a ticket, plus automatic command logging for all other ticket commands.

## SUBTASK: Define ticket ask behavior
- name: define-ticket-ask
- tags: planning
- description: Document expected CLI usage, file path, and output format.
- test-condition-1: expected usage includes optional --subtask flag, log command, and auto command logging
- test-condition-2: log.md path is defined in ticket folder
- status: todo

## SUBTASK: Implement ticket log commands
- name: implement-ticket-log
- tags: ticket, cli
- dependencies: define-ticket-ask
- description: Add `ticket ask` and `ticket log` CLI handling that appends to log.md.
- test-condition-1: log.md is created in src/tickets/<ticket>/ when missing
- test-condition-2: question entry includes timestamp and question text
- test-condition-3: log entry includes timestamp and log text
- test-condition-4: non-ask/log ticket commands append a command entry
- status: todo

## SUBTASK: Add integration tests
- name: add-integration-test
- tags: test
- dependencies: implement-ticket-log
- description: Extend ticket integration tests to cover ticket ask/log output and auto command logging.
- test-condition-1: integration test validates log.md content for questions
- test-condition-2: integration test validates log.md content for logs
- test-condition-3: integration test validates log.md command entry
- status: todo

## SUBTASK: Update README
- name: update-readme
- tags: docs
- dependencies: implement-ticket-log
- description: Document ticket ask/log usage and log.md in README.md.
- test-condition-1: README lists ticket ask and log commands
- test-condition-2: Ticket Structure lists log.md
- status: todo
