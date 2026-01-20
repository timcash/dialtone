# Plan: ticket-34

## Goal
Implement the autonomous developer loop and subagent integration in `dialtone-dev` as described in ticket #34 and the design document.

## Tests
- [ ] test_developer_skeleton: Verify `dialtone-dev developer` command is registered and prints help.
- [ ] test_subagent_command: Verify `dialtone-dev subagent` command exists and identifies opencode.
- [ ] test_ticket_ranking: Verify the developer loop can fetch and rank tickets by capability labels.
- [ ] test_feature_dir_setup: Verify subagent task files are correctly created in `features/` directory.
- [ ] test_full_loop_e2e: Run a mock end-to-end cycle from ticket selection to PR creation.

## Notes
- `developer` command coordinates high-level workflow.
- `subagent` command abstracts the LLM interface (opencode/Antigravity).
- Improved `AGENT.md` to reflect advanced agent capabilities.

## Blocking Tickets
- None

## Progress Log
- 2026-01-19: Created initial plan and design document.
