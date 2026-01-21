# Branch: geospatial-tools (do not use / in the branch name they become folders, only use -)
# Task: geospatial tools

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
![image](https://github.com/user-attachments/assets/c87b9dd0-7bfc-4451-b7e5-da9c6d8de46e)

integrate tools from this point cloud library for LIDAR data

https://github.com/opengeos/maplibre-gl-usgs-lidar

## Collaborative Notes
[A place for humans and the autocoder to share research, technical decisions, or state between context windows.]

## Tools and Tips for writing great tickets
1. Use this file as a template for writing tickets

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
