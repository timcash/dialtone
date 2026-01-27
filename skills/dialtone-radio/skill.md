# Dialtone Radio Skill
Dialtone Radio supports open radio field uplink for relaying real-time video and telemetry. It focuses on stable links, bandwidth efficiency, and resilient transport.

## Core Focus
- Provide uplink and downlink paths for field deployment.
- Optimize telemetry and video transmission over radio.
- Maintain reliability across varied RF conditions.

## Capabilities
- Relay telemetry and video over open radio hardware.
- Adjust bitrate and buffering for unstable links.
- Provide failover strategies for loss recovery.

## Inputs
- Radio hardware profiles and frequency plans.
- Bandwidth and latency constraints.
- Telemetry and video stream priorities.

## Outputs
- Uplink configuration and health metrics.
- Stream quality reports and tuning notes.
- Incident logs for connection drops.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for radio features.
- `docs/workflows/subtask_expand.md` for link tuning.
- `docs/workflows/issue_review.md` for RF issues.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh mavlink` and `./dialtone.sh logs` for transport and visibility.
- `./dialtone.sh vpn` for secure routing where available.
- `./dialtone.sh diagnostic` for RF health checks.

## Example Tasks
- Tune telemetry prioritization for low bandwidth.
- Add link health monitoring and alerts.
- Validate video relay under degraded conditions.

## Notes
- Keep failover paths documented and tested.
- Record RF assumptions for deployment sites.
