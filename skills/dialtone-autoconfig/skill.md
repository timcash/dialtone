# Dialtone Autoconfig Skill
Dialtone Autoconfig automates discovery and configuration for sensors, actuators, compute, storage, and networking. It targets plug-and-play setups across hardware and deployment environments.

## Core Focus
- Detect and configure hardware interfaces automatically.
- Allocate compute and storage for AI and telemetry.
- Provide zero-config networking and peer discovery.

## Capabilities
- Plug-and-play detection for cameras, IMUs, LIDAR, and audio.
- Unified control for PWM, stepper, and CAN-bus devices.
- Dynamic compute allocation for inference and video encoding.

## Inputs
- Hardware inventory and capability constraints.
- Desired performance targets and resource limits.
- Network environment and discovery rules.

## Outputs
- Generated configuration profiles and mappings.
- Validation reports for detected hardware.
- Resource allocation and tuning summaries.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for configuration tasks.
- `docs/workflows/subtask_expand.md` for device support.
- `docs/workflows/issue_review.md` for hardware regressions.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh camera`, `./dialtone.sh mavlink`, and `./dialtone.sh install` for device setup.
- `./dialtone.sh diagnostic` and `./dialtone.sh logs` for verification.
- `./dialtone.sh deploy` for rollout to devices.

## Example Tasks
- Add auto-detection for a new sensor driver.
- Tune resource allocation for on-device inference.
- Validate storage ring-buffer settings under load.

## Notes
- Document default mappings and overrides.
- Keep safe fallbacks for unknown devices.
