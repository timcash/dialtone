# Plan: github-ticket-command

## Goal
Implement a GitHub ticket management subcommand in the `dialtone-dev` CLI that wraps the `gh` tool for project-specific usage.

## Tests
- [x] test_ticket_help: Verify `dialtone-dev ticket` shows correct usage
- [x] test_ticket_list: Verify it lists tickets from the repo
- [ ] test_ticket_add: Verify interactive creation (manual)
- [ ] test_ticket_comment: Verify commenting on an ticket (manual)

## Notes
- Wraps `gh` CLI directly
- CLI must be installed and authenticated

## Blocking Tickets
- None

## Progress Log
- 2026-01-16: Created plan file
