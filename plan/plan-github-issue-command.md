# Plan: github-issue-command

## Goal
Implement a GitHub issue management subcommand in the `dialtone-dev` CLI that wraps the `gh` tool for project-specific usage.

## Tests
- [x] test_issue_help: Verify `dialtone-dev issue` shows correct usage
- [x] test_issue_list: Verify it lists issues from the repo
- [ ] test_issue_add: Verify interactive creation (manual)
- [ ] test_issue_comment: Verify commenting on an issue (manual)

## Notes
- Wraps `gh` CLI directly
- CLI must be installed and authenticated

## Blocking Issues
- None

## Progress Log
- 2026-01-16: Created plan file
