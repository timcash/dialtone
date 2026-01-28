# Name: ticket-ask-question
# Tags: ticket, plugin, docs

# Goal
Add a `ticket ask` command that writes `question.md` entries alongside a ticket, then cover it in tests and docs.

## SUBTASK: Define ticket ask behavior
- name: define-ticket-ask
- tags: planning
- description: Document expected CLI usage, file path, and output format.
- test-condition-1: expected usage includes optional --subtask flag
- test-condition-2: question.md path is defined in ticket folder
- status: todo

## SUBTASK: Implement ticket ask command
- name: implement-ticket-ask
- tags: ticket, cli
- dependencies: define-ticket-ask
- description: Add `ticket ask` CLI handling that appends to question.md.
- test-condition-1: question.md is created in src/tickets/<ticket>/ when missing
- test-condition-2: question entry includes timestamp and question text
- status: todo

## SUBTASK: Add integration test
- name: add-integration-test
- tags: test
- dependencies: implement-ticket-ask
- description: Extend ticket integration tests to cover ticket ask output.
- test-condition-1: integration test validates question.md content
- status: todo

## SUBTASK: Update README
- name: update-readme
- tags: docs
- dependencies: implement-ticket-ask
- description: Document ticket ask usage and question.md in README.md.
- test-condition-1: README lists ticket ask command
- test-condition-2: Ticket Structure lists question.md
- status: todo
