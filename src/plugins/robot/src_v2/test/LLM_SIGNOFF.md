# Robot src_v2 LLM Sign-Off Checklist

Use this file for every `src_v2` task handoff.

## Required Sign-Off Fields (per task)
- `local-test`: `done` or `not-done`
- `on-robot-test`: `done` or `not-done`
- `documents-update`: `done` or `not-done`

Each field must include evidence:
- commands run
- key output/result summary
- links/paths to artifacts (logs/screenshots)

## Task A - robot src_v2 runtime/webserver + embedded NATS updates
- `local-test`:
- `on-robot-test`:
- `documents-update`:

## Task B - robot src_v2 UI/module shape
- `local-test`:
- `on-robot-test`:
- `documents-update`:

## Task C - camera/mavlink integration
- `local-test`:
- `on-robot-test`:
- `documents-update`:

## Task D - autoswap + composition
- `local-test`:
- `on-robot-test`:
- `documents-update`:

## Final Mandatory UI E2E

### E2E-1 Local Mock Server
- Run UI locally with mock server and mock `/natsws` data.
- Validate critical UI paths and state updates.
- Record result:
  - status:
  - commands:
  - artifacts:

### E2E-2 Local UI + Remote Robot `/natsws`
- Run UI locally, connect to remote robot/repl `/natsws` feed.
- Validate live camera + mavlink data rendering from remote source.
- Record result:
  - status:
  - commands:
  - artifacts:

## Release Gate
`src_v2` is not ready unless all task sign-offs are complete and both mandatory UI E2E scenarios pass.
