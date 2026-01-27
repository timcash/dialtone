# Dialtone Geo Skill
Dialtone Geo delivers geospatial intelligence for terrain context, alerting, and fleet visualization. It combines satellite data with robot telemetry for location-aware operations.

## Core Focus
- Integrate terrain and environmental context.
- Trigger alerts from GPS boundaries and imagery updates.
- Visualize multi-robot fleets on maps.

## Capabilities
- Connect to geospatial datasets and services.
- Define geofences and alert policies.
- Render fleet status on 3D or 2D maps.

## Inputs
- GPS telemetry and mission boundaries.
- Map layers and satellite data sources.
- Alert rules and notification settings.

## Outputs
- Geospatial context overlays and alerts.
- Fleet map views and status summaries.
- Audit trails for boundary events.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for geo features.
- `docs/workflows/subtask_expand.md` for map layers.
- `docs/workflows/issue_review.md` for data issues.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh www` and `./dialtone.sh ui` for visualization.
- `./dialtone.sh logs` for audit trails.
- `./dialtone.sh deploy` for map service updates.

## Example Tasks
- Add a new geofence and alert channel.
- Integrate a terrain layer for route planning.
- Build a fleet overview page for operators.

## Notes
- Keep data sources documented with update cadence.
- Validate coordinate formats and projections.
