# Dialtone CLI Skill
Dialtone CLI focuses on the single-binary interface used to connect, control, and deploy to robots across platforms. It emphasizes reliable commands, packaging, and cross-platform operability.

## Core Focus
- Provide consistent CLI command design and UX.
- Build and deploy to devices such as ARM64 targets.
- Enable discovery, configuration, and remote operations.

## Capabilities
- Cross-platform builds for macOS, Linux, and Windows.
- One-command build and deploy workflows.
- Remote copy and bootstrap of the CLI binary.

## Inputs
- Target platform and architecture.
- Device connection details and credentials.
- Build configuration and feature flags.

## Outputs
- CLI binaries and build artifacts.
- Deployment logs and verification results.
- User-facing command documentation updates.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for CLI changes.
- `docs/workflows/issue_review.md` for user-reported bugs.
- `docs/workflows/subtask_expand.md` for command refactors.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh build`, `./dialtone.sh install`, and `./dialtone.sh deploy` for lifecycle operations.
- `./dialtone.sh logs` and `./dialtone.sh diagnostic` for troubleshooting.
- `./dialtone.sh plugin` and `./dialtone.sh ticket` for scaffolding and tracking.

## Example Tasks
- Add a new CLI command with tests and docs.
- Improve deploy reliability for ARM64 devices.
- Optimize build times for local development.

## Notes
- Keep commands deterministic and backward compatible.
- Document flags and defaults in CLI docs.
