# Documentation Standardization Summary: v2 Transition

This document outlines the changes made to move the repository from dual v1/v2 documentation to a single, standardized "v2" system.

## 1. Deletions (Legacy v1)
The following legacy v1 files were removed:
- `docs/workflows/issue_review_v1.md`
- `docs/workflows/subtask_expand_v1.md`
- `docs/workflows/ticket_v1.md`

## 2. Promotions (v2 to Primary)
The v2 workflow files were promoted to be the primary documentation by renaming them and removing the `_v2` suffix:
- `docs/workflows/issue_review_v2.md` -> `docs/workflows/issue_review.md`
- `docs/workflows/subtask_expand_v2.md` -> `docs/workflows/subtask_expand.md`
- `docs/workflows/ticket_v2.md` -> `docs/workflows/ticket.md`

The `.agent/workflows/` directory was also updated to reflect these primary versions.

## 3. Global Updates
- **README.md**: Removed "Legacy v1" sections and standardized the "Ticket Lifecycle" and "Ticket Structure" sections on the v2 system.
- **.agent/rules/rule-cli.md**: Removed legacy v1 command references.
- **Internal Cleanup**: Removed "(v2 specific)" and "(Standardized v2)" labels from all workflow headers and metadata for a cleaner, unified look.
- **Consistency**: Standardized CLI command examples to use the `./dialtone.sh github` syntax.
