# Issue Improvement: Design Prototype Integration of Code and Dialtone CLI (#34)

## Overview
This document outlines the design for the "developer loop" in `dialtone-dev`, which automates the manual workflow described in `AGENT.md`. The goal is to allow `dialtone-dev` to autonomously identify, solve, and submit improvements to the codebase.

## Current State vs. Proposed Automation
The current `dialtone-dev` provides the building blocks:
- `issue`: Manage GitHub issues via `gh`.
- `plan`: Create and track feature plans.
- `branch`: Manage git branches.
- `test`: Run various test types.
- `pull-request`: Create and update PRs.

The proposed automation will orchestrate these into a continuous loop.

## Proposed Commands

### 1. `dialtone-dev developer`
The main orchestrator for the autonomous loop.

**Workflow Logic:**
1. **Identify**: Fetch open issues using `dialtone-dev issue list`. Prioritize based on robot-specific labels (e.g., `camera`, `gpu`, `mavlink`).
2. **Setup**:
   - Pick the best issue.
   - Create a feature branch: `git checkout -b features/<issue-id>-<short-title>`.
   - Create a plan file in a new `features/` directory (or use `plan/`).
3. **Delegate**: Start a subagent with the specific task.
   - Command: `dialtone-dev subagent --task features/<branch-name>/task.md`
4. **Monitor**: Watch for completion or "stuck" signals from the subagent.
   - If stuck: Log the error and notify the operator (developer).
   - If complete: Proceed to submission.
5. **Submit**:
   - Run verification tests: `dialtone-dev test`.
   - Create a Pull Request: `dialtone-dev pull-request --body-file plan/plan-<name>.md`.
6. **Repeat**: Loop back to step 1.

### 2. `dialtone-dev subagent`
A specialized command to interface with an LLM for code changes.

**Options:**
- `--task <file>`: The task checklist (Markdown) for the agent to follow.
- `--context <dir>`: Relevant documentation or source files.

**Implementation detail:** This command should wrap `opencode` or any other agentic subagent (like Antigravity) that can perform file edits and run commands.

## Proposed File & Directory Structure
To support multiple concurrent features or automated runs, we should use a `features/` directory:
```
features/
└── <branch-name>/
    ├── task.md        # The specific task checklist for the subagent
    └── tests/          # Feature-specific tests created by the agent
```

## Monitoring & Assistance
The `developer` command should monitor the `opencode.log` or a specific stdout/stderr pipe from the subagent.
- **Heartbeat**: Subagent should update a `status: active` field in the task file.
- **Assistance**: If no progress is made for X minutes (configurable), the loop pauses and alerts the user.

## Verification & Test Loop
A dedicated test command for the loop itself:
`dialtone-dev test loop --repo <url> --issue-id <id>`
This will:
1. Create a "dummy" issue on the repo.
2. Run the `developer` command targeting that issue.
3. Verify that a PR is correctly created and closes the dummy issue.

## Next Steps
1. Implement the `developer` command skeleton in `src/dev.go`.
2. Define the subagent interface (initially wrapping `opencode`).
3. Create the `features/` directory management logic.
4. Set up the E2E test loop for the automated developer.
