# Branch: antigravity-ide-plugin
# Tags: enhancement, ide

# Goal
Create an `ide` plugin for Dialtone that provides specialized tools for interacting with the Antigravity IDE, specifically for viewing and tailing its internal logs.

## SUBTASK: Initialize the `ide` plugin
- name: ide-plugin-init
- description: Use the Dialtone CLI to scaffold the `ide` plugin.
- test-description: Verify the plugin directory structure exists.
- test-command: `ls src/plugins/ide/README.md`
- status: done

## SUBTASK: Implement Antigravity log discovery
- name: log-discovery
- description: Create logic to find the most recent Antigravity extension log file in `~/Library/Application Support/Antigravity/logs/`.
- test-description: Verify the log path indexer can find the active log file.
- test-command: `./dialtone.sh test ticket antigravity-ide-plugin --subtask log-discovery`
- status: done

## SUBTASK: Implement `ide antigravity logs` command
- name: logs-command
- description: Add the `antigravity logs` subcommand to the `ide` plugin that tails the discovered log file.
- test-description: Verify the command runs and outputs log lines.
- test-command: `./dialtone.sh test ticket antigravity-ide-plugin --subtask logs-command`
- status: done

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start antigravity-ide-plugin`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket antigravity-ide-plugin`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done antigravity-ide-plugin`
- status: done

## Collaborative Notes
- **Context**: 
    - Log Path Prefix: `/Users/tim/Library/Application Support/Antigravity/logs/`
    - Target Log: `*/exthost/google.antigravity/Antigravity.log`
- **Implementation Notes**: 
    - Need to handle multiple windows by choosing the one with the latest modification time.
- **Reference**: N/A


