---
trigger: model_decision
description: When working on a ticket, always do each subtask one at a time. Subtasks use this format.
---

When working on a ticket, always do each subtask one at a time. Subtasks use this format.

# Format

```markdown
## SUBTASK: Small 10 minute task title
- name: name-with-only-lowercase-and-dashes
- tags: comma, separated, tags
- dependencies: previous-subtask-name
- description: a single paragraph that guides the LLM to take a small testable step
- test-condition-1: a clear condition that must be met
- test-condition-2: another clear condition that must be met
- agent-notes: any notes about implementation or blockers
- pass-timestamp: ISO8601 timestamp on pass
- fail-timestamp: ISO8601 timestamp on fail
- status: one of four status values (todo|progress|done|failed)
```

# Example

```markdown
## SUBTASK: Install Video Driver Environment
- name: install-video-driver-environment
- description: write code to install V4L2 headers into the install cli tools
- test-description: run `dialtone.sh install` then verify `TestV4L2Headers` using `os.Stat`.
- test-command: `dialtone.sh test ticket video-driver-improvements`
- status: todo
```