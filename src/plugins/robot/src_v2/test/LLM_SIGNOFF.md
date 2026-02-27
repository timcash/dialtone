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
    - `/natsws` websocket dial succeeds against robot `src_v2` embedded NATS WS bridge
    - `/stream` returned scaffold `503` (expected until camera wiring lands)
    - `robot src_v2 test` passed with `src/plugins/test/src_v1` registry orchestration
    - steps use `WaitForStepMessageAfterAction` for action -> wait assertions
  - artifacts:
    - `/tmp/robot_v2_server.log`
    - `/tmp/robot_v2_root.out`
- `on-robot-test`: done
  - commands:
    - `./dialtone.sh robot src_v2 sync-code --host rover-1.shad-artichoke.ts.net --user tim`
    - `ssh tim@rover-1.shad-artichoke.ts.net 'cd ~/dialtone && ./dialtone.sh robot src_v2 test'`
  - result summary:
    - rover test suite passed end-to-end for Task A smoke coverage
    - `/health` ok, `/api/init` returns wsPath, `/natsws` websocket connect succeeded, `/stream` scaffold 503 expected
- `documents-update`: done
  - changed docs:
    - `src/plugins/robot/src_v2/v2.md`
    - `src/plugins/robot/src_v2/test/LLM_SIGNOFF.md`

## Task B - robot src_v2 UI/module shape
- `local-test`: done (parity scaffold phase 1)
  - commands:
    - `rsync -a --delete --exclude node_modules --exclude dist src/plugins/robot/src_v1/ui/ src/plugins/robot/src_v2/ui/`
    - `./dialtone.sh robot src_v2 install`
    - `./dialtone.sh robot src_v2 build`
  - result summary:
    - `src_v2/ui` scaffolded from `src_v1/ui` for parity-first baseline
    - local Vite production build succeeded for `src_v2/ui`
- `on-robot-test`: done
  - commands:
    - `./dialtone.sh robot src_v2 sync-code --host rover-1.shad-artichoke.ts.net --user tim`
    - `ssh tim@rover-1.shad-artichoke.ts.net 'cd ~/dialtone && ./dialtone.sh robot src_v2 install && ./dialtone.sh robot src_v2 build'`
  - result summary:
    - rover `src_v2` UI dependency install succeeded
    - rover `src_v2` Vite production build succeeded
- `documents-update`: done
  - changed docs:
    - `src/plugins/robot/src_v2/test/LLM_SIGNOFF.md`

## Task C - camera/mavlink integration
- `local-test`: done (integration-health scaffold phase 1)
  - commands:
    - `./dialtone.sh robot src_v2 test`
  - result summary:
    - `robot src_v2` smoke suite passed locally with `/api/integration-health` assertion
    - response includes scaffold degraded status with `camera` and `mavlink` marked `not-configured`
- `on-robot-test`: done
  - commands:
    - `./dialtone.sh robot src_v2 sync-code --host rover-1.shad-artichoke.ts.net --user tim`
    - `ssh tim@rover-1.shad-artichoke.ts.net 'cd ~/dialtone && ./dialtone.sh robot src_v2 test'`
  - result summary:
    - rover smoke suite passed with `/api/integration-health` assertion
    - `/natsws` websocket connect still passes in rover environment
- `documents-update`: done
  - changed docs:
    - `src/plugins/robot/src_v2/test/LLM_SIGNOFF.md`

## Task D - autoswap + composition
- `local-test`: done (manifest contract + compose run phase 2)
  - commands:
    - `./dialtone.sh robot src_v2 test`
    - `./dialtone.sh autoswap src_v1 test`
  - result summary:
    - local smoke suite passed including `03-manifest-has-required-sync-artifacts`
    - local smoke suite passed `05-autoswap-compose-run-smoke`
    - manifest asserts required sync keys: autoswap, robot, repl, camera, mavlink, wlan, ui_dist
    - autoswap compose run validated staged artifacts + robot/ui + camera/mavlink heartbeat sidecars
- `on-robot-test`: done
  - commands:
    - `./dialtone.sh robot src_v2 sync-code --host rover-1.shad-artichoke.ts.net --user tim`
    - `ssh tim@rover-1.shad-artichoke.ts.net 'cd ~/dialtone && ./dialtone.sh robot src_v2 test'`
  - result summary:
    - rover smoke suite passed including manifest sync-artifact contract step
- `documents-update`: done
  - changed docs:
    - `src/plugins/robot/src_v2/test/LLM_SIGNOFF.md`

## Final Mandatory UI E2E

### E2E-1 Local Mock Server
- Run UI locally with mock server and mock `/natsws` data.
- Validate critical UI paths and state updates.
- Record result:
  - status: done
  - commands:
    - `DIALTONE_TEST_BROWSER_NODE=chroma ./dialtone.sh robot src_v2 test`
  - artifacts:
    - `src/plugins/robot/src_v2/test/TEST.md` (all 5 steps passed)
    - Browser console evidence in test output includes:
      - `Connecting to ws://legion-wsl-1.shad-artichoke.ts.net:18083/natsws...`
      - `Connected.`
      - menu navigation actions + section activation checks
      - `mock nats publish ok`

### E2E-2 Local UI + Remote Robot `/natsws`
- Run UI locally, connect to remote robot/repl `/natsws` feed.
- Validate live camera + mavlink data rendering from remote source.
- Record result:
  - status: done
  - commands:
    - `DIALTONE_TEST_BROWSER_NODE=chroma DIALTONE_TEST_BROWSER_BASE_URL=https://rover-1.dialtone.earth ./dialtone.sh robot src_v2 test`
  - artifacts:
    - `src/plugins/robot/src_v2/test/TEST.md` (all 5 steps passed)
    - Browser console evidence in test output includes:
      - `Connecting to wss://rover-1.dialtone.earth/natsws...`
      - `Connected.`
      - full section/menu traversal + aria checks passed

## Release Gate
`src_v2` is not ready unless all task sign-offs are complete and both mandatory UI E2E scenarios pass.
