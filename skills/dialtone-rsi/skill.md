# Dialtone RSI Skill
Dialtone RSI (Realtime Strategy Interface) enables collaborative mission planning and live command overrides for robot swarms. It focuses on shared situational awareness and safe coordination.

## Core Focus
- Provide strategic dashboards for multi-robot operations.
- Enable shared mission planning and execution.
- Support real-time command overrides by operators.

## Capabilities
- Drag-and-drop mission planning interfaces.
- Live coordination views for fleets and squads.
- Operator override controls with audit trails.

## Inputs
- Fleet status, telemetry, and task queues.
- Mission plans and operator roles.
- Override policies and escalation rules.

## Outputs
- Updated mission plans and execution states.
- Command override records and alerts.
- Fleet coordination reports.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for RSI features.
- `docs/workflows/subtask_expand.md` for UI flows.
- `docs/workflows/issue_review.md` for coordination bugs.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh ui` and `./dialtone.sh www` for dashboard delivery.
- `./dialtone.sh logs` for auditing.
- `./dialtone.sh diagnostic` for system health checks.

## Example Tasks
- Add a new mission planning widget.
- Build an operator override confirmation flow.
- Improve fleet status aggregation performance.

## Notes
- Always log overrides with operator identity.
- Keep latency low for live coordination views.
