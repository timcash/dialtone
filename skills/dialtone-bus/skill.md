# Dialtone Bus Skill
Dialtone Bus handles scalable command and control data structures, enabling request/reply and streaming patterns across systems. It emphasizes reliability, fanout, and replayable telemetry.

## Core Focus
- Implement request/reply command patterns.
- Support fanout, queuing, and load balancing.
- Enable streaming and replay of telemetry and video.

## Capabilities
- Route commands across services with consistent schemas.
- Buffer and replay data for analysis or recovery.
- Scale data flows across multi-robot fleets.

## Inputs
- Message schemas and routing rules.
- Telemetry and video stream definitions.
- Load targets and performance constraints.

## Outputs
- Verified command pipelines and stream configs.
- Metrics on throughput, latency, and reliability.
- Documentation for message contracts.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for bus changes.
- `docs/workflows/subtask_expand.md` for schema evolution.
- `docs/workflows/issue_review.md` for reliability bugs.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh mavlink` and `./dialtone.sh logs` for data transport and audit.
- `./dialtone.sh diagnostic` for health checks.
- `./dialtone.sh test` for pipeline verification.

## Example Tasks
- Add a new telemetry stream with replay support.
- Optimize queue behavior under bursty traffic.
- Define a command schema for a new actuator.

## Notes
- Keep schemas stable; version changes carefully.
- Measure latency for real-time control paths.
