# Plan: Rename ticket to ticket and Standardize Subtasks

This plan outlines the steps to migrate all references and functionality of `ticket` to simply `ticket`, and to standardize the formatting of `## SUBTASK` blocks in all markdown files, following `src/plugins/ticket/README.md` as the source of truth.

## Proposed Changes

### Core System
- [MODIFY] `dialtone.sh`: Update commands from `ticket` to `ticket`.
- [NEW] `src/plugins/ticket`: Move all logic from `src/plugins/ticket` here.
- [DELETE] `src/plugins/ticket`: Remove after migration.

### Source Code
- Update all Go imports and references in `src/` from `plugins/ticket` to `plugins/ticket`.
- Replace string literals `"ticket"` with `"ticket"` where appropriate.

### Documentation
- Update all `.md` files to replace `ticket` with `ticket`.
- Update `docs/ticket-template.md` to the new `## SUBTASK` format.
- Update all existing `ticket.md` files (and any other md files with subtasks) to the new format from the source of truth:
  ```markdown
  ## SUBTASK: [Human Readable Name]
  - name: [slug-name]
  - tags: [tags]
  - dependencies: [dependencies]
  - description: [description]
  - test-condition-1: [test-condition-1]
  - test-condition-2: [test-condition-2]
  - agent-notes: [agent-notes]
  - pass-timestamp: [pass-timestamp]
  - fail-timestamp: [fail-timestamp]
  - status: [status]
  ```

## Verification Plan

### Automated Tests
- Run `grep -r "ticket" .` to ensure no references remain.
- Run `grep -r "## SUBTASK" .` and verify the format of matches.
- Execute `./dialtone.sh ticket list` to verify the CLI still works after renaming.

### Manual Verification
- Verify that `src/plugins/ticket/README.md` correctly describes the system as `ticket`.
- Spot check a few migrated `ticket.md` files for correct field mapping.
