# Dialtone Mock Mode Skill
Dialtone Mock Mode enables hardware-free development of the dashboard UI and telemetry pipelines. It simulates sensors, video, and system signals to keep the dev loop fast and reliable.

## Core Focus
- Run UI and data pipelines without physical hardware.
- Generate realistic mock telemetry and video streams.
- Disable heavy drivers and AI features to speed iteration.

## Capabilities
- Provide fake heartbeat, attitude, GPS, and video feeds.
- Toggle mock execution for predictable test runs.
- Support UI and workflow validation in a lightweight mode.

## Inputs
- Mock mode flags and environment options.
- Desired telemetry profiles or ranges.
- UI feature targets for validation.

## Outputs
- Deterministic mock streams and status data.
- Debuggable UI state and reproducible demos.
- Notes on differences from hardware mode.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for UI or pipeline tasks.
- `docs/workflows/subtask_expand.md` for mock data coverage.
- `docs/workflows/issue_review.md` for UI regressions.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh ui` for dashboard behavior.
- `./dialtone.sh camera` and `./dialtone.sh mavlink` for simulated IO boundaries.
- `./dialtone.sh logs` and `./dialtone.sh test` for validation and snapshots.

## Example Tasks
- Validate UI flows using simulated telemetry.
- Reproduce a camera pipeline issue in mock mode.
- Build a demo sequence with consistent sensor playback.

## Notes
- Activate with `dialtone start --mock` when hardware is absent.
- Document deviations between mock and hardware paths.
