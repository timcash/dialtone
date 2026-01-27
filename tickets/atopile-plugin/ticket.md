# Branch: atopile-plugin
# Tags: p1, enhancement, hardware-design

# Goal
Integrate atopile into a Dialtone plugin to allow for describing electronics with code and leveraging software development workflows for hardware design.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start atopile-plugin`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket atopile-plugin`
- status: done

## SUBTASK: Research atopile integration
- name: research-atopile
- description: Research atopile's Python API and CLI requirements to define the plugin interface.
- test-description: Document key commands and dependencies in the collaborative notes.
- test-command: `ls tickets/atopile-plugin/atopile-research.md`
- status: todo

## SUBTASK: Scaffold atopile plugin
- name: scaffold-plugin
- description: Create the `atopile` plugin structure in `src/plugins/atopile`.
- test-description: Verify the plugin builds and is recognized by Dialtone.
- test-command: `./dialtone.sh plugin build atopile`
- status: todo

## SUBTASK: Implement atopile CLI wrappers
- name: integrate-atopile-cli
- description: Implement Dialtone CLI commands that wrap atopile's core functionality (e.g., compile, build).
- test-description: Verify that `dialtone.sh atopile build` correctly invokes the atopile CLI.
- test-command: `dialtone.sh test ticket atopile-plugin --subtask integrate-atopile-cli`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done atopile-plugin`
- status: todo

## Collaborative Notes
- **Reference**: https://github.com/timcash/dialtone/issues/55
- **atopile Documentation**: https://docs.atopile.io/atopile/introduction

