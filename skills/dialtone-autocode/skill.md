# Dialtone Autocode Skill
Dialtone Autocode focuses on system-tuned code generation and live code adaptation for the Dialtone CLI and robot control stack. It guides safe, traceable modifications to improve behavior, performance, and extensibility.

## Core Focus
- Generate and refine code with human-in-the-loop feedback.
- Adapt implementation details to new constraints or hardware.
- Produce context-aware scaffolding for new plugins and control logic.
- Share reproducible Autocode sessions for review and collaboration.

## Capabilities
- Modify source code to fix bugs or add features.
- Provide context-aware code suggestions for robot plugins.
- Create automation flows that are repeatable and testable.

## Inputs
- Target module or plugin names.
- Desired behavior, constraints, and success criteria.
- Optional telemetry samples or logs for debugging context.

## Outputs
- Code changes, patches, or generated scaffolding.
- Validation notes and test guidance.
- Shareable Autocode session artifacts or summaries.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for structured TDD execution.
- `docs/workflows/issue_review.md` for triage and review.
- `docs/workflows/subtask_expand.md` for decomposition.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh plugin` for scaffolding and discovery.
- `./dialtone.sh ticket` for structured task execution.
- `./dialtone.sh github` for issue and PR linkage.
- `./dialtone.sh ai`, `./dialtone.sh go`, and `./dialtone.sh test` for implementation and validation.

## Example Tasks
- Generate a new plugin scaffold and wire it into the CLI.
- Refactor a module to improve performance under load.
- Implement a bug fix with a matching regression test.

## Notes
- Keep edits small and testable; prefer incremental changes.
- Use Mock Mode when hardware is not available.
