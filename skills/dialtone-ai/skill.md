# Dialtone AI Skill
Dialtone AI enables vision and LLM-assisted operation for navigation, diagnostics, and natural language control. It focuses on reliable inference, safe actions, and explainable outputs.

## Core Focus
- Run real-time vision detection and tracking.
- Interpret natural language commands safely.
- Analyze telemetry anomalies using LLMs.

## Capabilities
- Object detection and tracking for navigation.
- Command parsing for natural language requests.
- Automated troubleshooting of system anomalies.

## Inputs
- Camera feeds or sensor telemetry.
- Command intents and safety constraints.
- Model configurations and performance targets.

## Outputs
- Actionable navigation or control commands.
- Diagnostic summaries and recommended fixes.
- Model performance and confidence metrics.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for AI feature work.
- `docs/workflows/subtask_expand.md` for model updates.
- `docs/workflows/issue_review.md` for AI regressions.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh ai` for model orchestration.
- `./dialtone.sh camera` and `./dialtone.sh mavlink` for sensor inputs.
- `./dialtone.sh logs` and `dialtone-dev test` for evaluation.

## Example Tasks
- Add a new vision model for object tracking.
- Improve command grounding and safety checks.
- Diagnose inference latency on target hardware.

## Notes
- Prefer interpretable outputs with confidence signals.
- Keep safe stop behavior for uncertain commands.
