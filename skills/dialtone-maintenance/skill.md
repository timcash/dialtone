# Dialtone Maintenance Skill
Dialtone Maintenance covers supply chain coordination, assembly guidance, and ongoing repair workflows. It targets reliable updates, diagnostics, and service operations.

## Core Focus
- Provide assembly and maintenance instructions.
- Automate software updates and diagnostics.
- Coordinate repair workflows and supply chain needs.

## Capabilities
- Generate step-by-step maintenance procedures.
- Run automated health checks and diagnostics.
- Schedule updates and verify deployment success.

## Inputs
- Hardware inventory and service history.
- Diagnostic logs and error reports.
- Update policies and maintenance schedules.

## Outputs
- Maintenance guides and checklists.
- Repair recommendations and parts lists.
- Update reports and health summaries.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for maintenance tasks.
- `docs/workflows/subtask_expand.md` for repair steps.
- `docs/workflows/issue_review.md` for failure analysis.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh diagnostic` and `./dialtone.sh logs` for health checks.
- `./dialtone.sh install` and `./dialtone.sh deploy` for updates.
- `./dialtone.sh ticket` for tracking maintenance work.

## Example Tasks
- Create a maintenance checklist for a new robot model.
- Run a diagnostic sweep and summarize failures.
- Automate update rollout with verification steps.

## Notes
- Record serials and revision details on each service.
- Keep rollback steps documented for updates.
