---
trigger: model_decision
description: When working on a ticket, always do each subtask one at a time. Subtasks use this format.
---
1. turn all prompts or issues into subtasks
1. write the subtask TEST first 
1. then write the code to pass the test
1. try to use only `dialtone.sh` and `git` commands
1. DO NOT run multiple cli commands in one line e.g. `dialtone.sh deploy`
1. exceptions for searching code and the web and editing code directly are acceptable
1. guide all work into `dialtone.sh ticket subtask` commands with a test at the end
1. build plugins if you do not see one you think fits this subtask
1. you may REORDER subtasks if needed
1. use the `dialtone.sh help` to print the help menu

# Good Subtask Title Examples:
- Integrate opencode and robot ui xterm element
- Allow the robot rover web ui to stream the opencode cli into xterm.js
- Search the code base for the web ui that gets deployed to the robot
- Look at the webpage interface that comes with opencode
- Add a new test for the video driver improvements
- Remove old logging code and update to the new logger.go package

When working on a ticket, always do each subtask one at a time. Subtasks use this format.

# Format

```markdown
## SUBTASK: Small 10 minute task title
- name: name-with-only-lowercase-and-dashes
- description: a single paragraph that guides the LLM to take a small testable step
- test-description: a suggestion that the LLM can use on how to test this change works
- test-command: the actual command to run the test in `dialtone.sh <test-command>` format
- status: one of three status values (todo|progress|done)
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