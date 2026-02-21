# Task Plugin

The `task` plugin provides a structured system for managing and tracking engineering tasks using Markdown-based data and a versioned workflow (`v1` vs `v2`).

## Core Concepts

### 1. The v1/v2 Workflow
To ensure clarity between "where we started" and "what we changed," each task exists in two versions:
- **`v1` (Baseline):** The state of the task at the beginning of the current work cycle (e.g., at the start of a git commit or LLM session).
- **`v2` (WIP):** The current working state. All updates, signatures, and status changes are recorded here.

### 2. Task Markdown Format
Tasks are defined in `.md` files with a specific structure:
- **`### task-dependencies:`** List of other task IDs this task depends on.
- **`### reviewed:`** Signatures from reviewers.
- **`### tested:`** Signatures from testers.
- **`### test-command:`** The command to run to verify the task.

## CLI Commands

Manage tasks via `./dialtone.sh task <command>`:

### `create <task-name>`
Scaffolds a new task in `src/plugins/task/database/<task-name>/v1/`.
Copies `v1` to `v2` to initialize the working state.

### `validate <task-name> <version>`
Validates the format of the task markdown file (e.g., `src_v1` or `src_v2`).

### `sign <task-name> <version> --role <role>`
Adds a signature to the `reviewed` or `tested` section of the task.

### `status <task-name>`
Shows the current status, dependencies, and diff between `v1` and `v2`.

### `archive <task-name>`
Promotes `v2` to `v1` to prepare for the next work cycle:
1. `rm -rf v1`
2. `mv v2 v1`
3. `cp -r v1 v2`
After this, `v1` and `v2` match, providing a clean baseline for the next agent.

## Implementation Details

### Directory Structure
```text
src/plugins/task/
  database/
    <task-name>/
      v1/
        <task-name>.md
      v2/
        <task-name>.md
```

## Example Workflow

1. **Start a new task:**
   ```sh
   ./dialtone.sh task create auth-fix
   ```

2. **Work on the task:**
   Edit `src/plugins/task/database/auth-fix/v2/auth-fix.md` or use CLI to update it.

3. **Verify and Sign:**
   ```sh
   ./dialtone.sh task sign auth-fix v2 --role LLM-CODE
   ./dialtone.sh task sign auth-fix v2 --role LLM-TEST
   ```

4. **Prepare for handoff:**
   ```sh
   ./dialtone.sh task archive auth-fix
   ```

## Verification

The task system itself is verified via:
- `./dialtone.sh task test src_v1`
