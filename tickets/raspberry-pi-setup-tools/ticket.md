# Branch: raspberry-pi-setup-tools (do not use / in the branch name they become folders, only use -)
# Task: Raspberry Pi Setup Tools

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create <plugin-name>` to create a new plugin if this ticket is about a plugin
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests files in `ticket/<ticket-name>/test/` to drive all work
2. [item 2]
3. [item 3]

## Non-Goals
1. DO NOT use manual shell commands to verify functionality.
2. DO NOT [item 2]
3. DO NOT [item 3]

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh ticket test <ticket-name>
   ```
2. **Plugin Tests**: If this ticket involves a plugin, run its specific tests.
   ```bash
   ./dialtone.sh plugin test <plugin-name>
   ```
3. **Feature Tests**: Run tests for a specific feature, which searches through tickets, plugins, and core tests.
   ```bash
   ./dialtone.sh test <feature-name>
   ```
4. **All Tests**: Run the entire test suite (core, plugins, and tickets).
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use the `src/logger.go` package to log messages.
2. Use logs to help debug and understand the code.

## Subtask: Research
- description: [List files to explore, documentation to read, or concepts to understand]
- test: [How to verify this research informed the work]
- status: todo

## Subtask: Implementation
- description: [NEW/MODIFY] [file_path]: [Short description of change]
- test: [What test or check proves this subtask works]
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: [Expected outcome or artifact to confirm success]
- status: todo


## Issue Summary
1. with the correct SSID and password to join wifi\n2. turn off blue tooth with the config file\n\n```\nCheck the presence of the parameters enable_uart=1 and dtoverlay=pi 3-disable-bt in the file /boot/config.txt by running the following command on the Raspberry Pi:\n\n cat /boot/config.txt | grep -E "^enable_uart=.|^dtoverlay=pi3-disable-bt"\n```\n3. set the hostname\n4. add a robot user

## Collaborative Notes
[A place for humans and the autocoder to share research, technical decisions, or state between context windows.]

## Tools and Tips for writing great tickets
1. Use this file as a template for writing tickets

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
