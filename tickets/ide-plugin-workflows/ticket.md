# Branch: ide-plugin-workflows
# Tags: <labels> (Must match GitHub labels: p0, bug, ready, enhancement, etc.)

# Goal
Create an `ide` plugin for Dialtone that provides a command to softlink files from `docs/workflows` to `.agent/workflows`. This helps in keeping the agent's workflows in sync with the project's documentation.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start ide-plugin-workflows`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket ide-plugin-workflows`
- status: done

## SUBTASK: add ide plugin boilerplate
- name: add-ide-plugin
- description: use the plugin cli to create the ide plugin structure
- test-description: verify src/plugins/ide directory and files exist
- test-command: `ls src/plugins/ide/README.md`
- status: done

## SUBTASK: add ide plugin to cli dispatcher
- name: register-ide-plugin
- description: update src/dev.go to include the ide plugin command
- test-description: verify dialtone.sh ide help shows the ide plugin usage
- test-command: `./dialtone.sh ide help`
- status: done

## SUBTASK: implement setup-workflows command
- name: implement-setup-workflows
- description: add setup-workflows command to ide plugin that creates softlinks from docs/workflows to .agent/workflows
- test-description: verify .agent/workflows contains softlinks to docs/workflows
- test-command: `./dialtone.sh ide setup-workflows && ls -l .agent/workflows/ticket.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `./dialtone.sh ticket done ide-plugin-workflows`
- status: done

## Collaborative Notes
- **Context**: Link relevant files (e.g., `[file.go](file:///path/to/file.go)`)
- **Implementation Notes**: Document technical decisions or blockers here.
- **Reference**: https://github.com/timcash/dialtone/issues/<id>

