---
trigger: always_on
---

Do each subtask one at a time. Subtasks use this format.

# Format

```markdown
## SUBTASK: Small 10 minute task title
- description: a single paragraph that guides the LLM to take a small testable step
- test-description: a suggestion that the LLM can use on how to test this change works
- test-command: the actual command to run the test in `dialtone.sh <test-command>` format
- status: one of three status values (todo|progress|done)
```

# Example

```markdown
## SUBTASK: Install Video Driver Environment
- description: write code to install V4L2 headers into the install cli tools
- test-description: run `dialtone.sh install` then verify `TestV4L2Headers` using `os.Stat`.
- test-command: `dialtone.sh ticket test video-driver-improvements`
- status: todo
```