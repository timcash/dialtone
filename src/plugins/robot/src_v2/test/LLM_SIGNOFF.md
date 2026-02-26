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
- `local-test`: done (scaffold phase 1)
  - commands:
    - `./dialtone.sh go src_v1 exec build -o ../bin/dialtone_robot_v2 ./plugins/robot/src_v2/cmd/server/main.go`
    - `./bin/dialtone_robot_v2 --listen :18082`
    - `curl http://127.0.0.1:18082/health`
    - `curl -o /tmp/robot_v2_root.out -w '%{http_code}' http://127.0.0.1:18082/`
    - `curl http://127.0.0.1:18082/api/init`
    - `curl -i http://127.0.0.1:18082/natsws`
    - `curl -i http://127.0.0.1:18082/stream`
    - `./dialtone.sh robot src_v2 test`
  - result summary:
    - `/health` returned `ok`
    - `/` returned `503` as expected when `ui/dist` is not configured yet
    - `/api/init` returned scaffold JSON with `wsPath=/natsws`
    - `/natsws` and `/stream` returned scaffold `503` (expected until bridge/camera wiring lands)
    - `robot src_v2 test` passed with `src/plugins/test/src_v1` registry orchestration
    - steps use `WaitForStepMessageAfterAction` for action -> wait assertions
  - artifacts:
    - `/tmp/robot_v2_server.log`
    - `/tmp/robot_v2_root.out`
- `on-robot-test`: not-done
  - reason:
    - Task A currently at local scaffold stage; remote robot validation deferred until `/natsws` and robot-specific endpoints are implemented.
- `documents-update`: done
  - changed docs:
    - `src/plugins/robot/src_v2/v2.md`
    - `src/plugins/robot/src_v2/test/LLM_SIGNOFF.md`

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
