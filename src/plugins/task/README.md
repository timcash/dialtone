# Task Plugin

The `task` plugin provides a structured system for managing and tracking engineering tasks using Markdown-based data and a versioned workflow (`v1` vs `v2`).

## Core Concepts

### 1. The v1/v2 Workflow
To ensure clarity between "where we started" and "what we changed," each task exists in two versions:
- **`v1` (Baseline):** The state of the task at the beginning of the current work cycle (e.g., at the start of a git commit or LLM session).
- **`v2` (WIP):** The current working state. All updates, signatures, and status changes are recorded here.

### 2. Task Markdown Format
Tasks are defined in `.md` files with a specific structure. There is exactly one H1 header (`#`) per file for the task title.

Required sections:
- **`### description:`** Actionable summary of the goal.
- **`### tags:`** Metadata for categorization.
- **`### task-dependencies:`** List of other task IDs this task depends on.
- **`### documentation:`** Reference URLs or local file paths.
- **`### test-condition-1:`** Verifiable criteria for success.
- **`### test-command:`** The command to run to verify the task.
- **`### reviewed:`** Signatures from reviewers (managed via CLI).
- **`### tested:`** Signatures from testers (managed via CLI).

## CLI Commands

Manage tasks via `./dialtone.sh task <command>`:

### `create <task-name>`
Scaffolds a new task in `src/plugins/task/database/<task-name>/v1/`.
Initializes `v1` and `v2` as baseline and working copies.

### `validate <task-name>`
Validates the format of the task markdown file in `v2`.

### `sign <task-name> --role <role>`
Adds a signature to the `reviewed` or `tested` section of the task in `v2`.

### `archive <task-name>`
Promotes `v2` to `v1` to prepare for the next work cycle. After this, `v1` and `v2` match.

## How to Build a Task

While you can use `./dialtone.sh task create`, you can also build tasks manually or via automation:

1. **Define the ID:** Choose a slugified ID (e.g., `my-feature-fix`).
2. **Create the Folder:** `mkdir -p src/plugins/task/database/my-feature-fix/{v1,v2}`.
3. **Draft the Markdown:** Create `my-feature-fix.md` in both `v1/` and `v2/`.
4. **Populate Sections:**
   - Ensure you use `### section-name:` for all headers.
   - Use `- none` instead of comments for empty lists.
   - Avoid multiple H1 headers; only the title uses `#`.
5. **Set the Baseline:** If you are migrating an issue, `v1` and `v2` should start as identical copies of the task.
6. **Link Dependencies:** Reference other task folders by their ID.

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
   ./dialtone.sh task sign auth-fix --role LLM-CODE
   ./dialtone.sh task sign auth-fix --role LLM-TEST
   ```

4. **Prepare for handoff:**
   ```sh
   ./dialtone.sh task archive auth-fix
   ```

## Verification

The task system itself is verified via:
- `./dialtone.sh task test src_v1`
