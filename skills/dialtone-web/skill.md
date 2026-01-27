# Dialtone Web Skill
Dialtone Web provides public robot presence, live status pages, and browser-based control. It emphasizes low-latency streaming and reliable public URLs.

## Core Focus
- Serve public robot URLs and status views.
- Enable low-latency WebRTC streaming.
- Support browser-based remote control interactions.

## Capabilities
- Publish public pages at `https://<robot_id>.dialtone.earth`.
- Stream live video and telemetry to browsers.
- Provide interactive controls with safe fallbacks.

## Inputs
- Robot IDs and access policies.
- WebRTC stream configurations.
- UI content and status metadata.

## Outputs
- Live public pages with status indicators.
- Streaming sessions and connectivity metrics.
- Access logs and usage analytics.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for web changes.
- `docs/workflows/issue_review.md` for web regressions.
- `docs/workflows/subtask_expand.md` for UI features.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh www` and `./dialtone.sh ui` for web UI delivery.
- `./dialtone.sh camera` for video inputs.
- `./dialtone.sh logs` for session diagnostics.

## Example Tasks
- Add a status widget to the public page.
- Improve WebRTC connection stability.
- Create a new public view for fleet status.

## Notes
- Keep public exposure minimal by default.
- Document access and privacy behavior clearly.
