# Test Report: repl-src-v3

- **Date**: Wed, 18 Mar 2026 21:26:53 PDT
- **Total Duration**: 5.022223973s

## Summary

- **Steps**: 2 / 2 passed
- **Status**: PASSED

## Details

### 1. ✅ service-start-publishes-heartbeat-and-service-registry-state

- **Duration**: 5.018076593s
- **Report**: Started named service pm-svc as pid 979556 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.

#### Logs

```text
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:52.658664608Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:52.658686794Z"}
DEBUG: [REPL][OUT] DIALTONE> Verifying dependencies...
DEBUG: [REPL][OUT] DIALTONE> Bootstrap path checks:
DEBUG: [REPL][OUT] DIALTONE> - repo root: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo (dir)
DEBUG: [REPL][OUT] DIALTONE> - src root: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/src (dir)
DEBUG: [REPL][OUT] DIALTONE> - env dir: /tmp/dialtone-repl-v3-bootstrap-3639358475/dialtone_env (dir)
DEBUG: [REPL][OUT] DIALTONE> - env json: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json (file)
DEBUG: [REPL][OUT] DIALTONE> - mesh config: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json (file)
DEBUG: [REPL][OUT] DIALTONE> Using managed Go (Cached): /tmp/dialtone-repl-v3-bootstrap-3639358475/dialtone_env/go/bin/go
DEBUG: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-19T04:26:53.8152294Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","timestamp":"2026-03-19T04:26:53.815331537Z"}
DEBUG: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
DEBUG: [REPL][OUT] DIALTONE> llm-codex joined room index (version=dev).
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.815554353Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.815557867Z"}
DEBUG: [REPL][OUT] DIALTONE> Leader active on DIALTONE-SERVER
DEBUG: [REPL][OUT] DIALTONE> Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)
DEBUG: [REPL][OUT] DIALTONE> Starting test: service-start-publishes-heartbeat-and-service-registry-state
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"lifecycle","message":"Starting test: service-start-publishes-heartbeat-and-service-registry-state","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Shared REPL session ready for llm-codex in room index.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Shared REPL session ready for llm-codex in room index.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Checking required files.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Checking required files.
DEBUG: [REPL][OUT] DIALTONE> repo root: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo (dir)
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"repo root: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo (dir)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> src/dev.go: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/src/dev.go (file)
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"src/dev.go: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/src/dev.go (file)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"env/dialtone.json: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json (file)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> env/dialtone.json: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json (file)
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Checking runtime variables.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Checking runtime variables.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_REPO_ROOT: set=true value=/tmp/dialtone-repl-v3-bootstrap-3639358475/repo","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_REPO_ROOT: set=true value=/tmp/dialtone-repl-v3-bootstrap-3639358475/repo
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_ENV_FILE: set=true value=/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_ENV_FILE: set=true value=/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/env/dialtone.json
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_REPL_NATS_URL: set=true value=nats://127.0.0.1:46222
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_REPL_NATS_URL: set=true value=nats://127.0.0.1:46222","room":"index","scope":"index","type":"line"}
INFO: [REPL][STEP 1] send="/service-start --name pm-svc -- proc src_v1 sleep 30" expect_room=6 expect_output=6 timeout=30s
INFO: [REPL][INPUT] /service-start --name pm-svc -- proc src_v1 sleep 30
DEBUG: [REPL][OUT] DIALTONE> dialtone.json metadata: mesh_nodes=0 names=none tailscale_keys=false cloudflare_keys=false
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"dialtone.json metadata: mesh_nodes=0 names=none tailscale_keys=false cloudflare_keys=false","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-start --name pm-svc -- proc src_v1 sleep 30","timestamp":"2026-03-19T04:26:53.818066825Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-start --name pm-svc -- proc src_v1 sleep 30","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.818282515Z"}
DEBUG: [REPL][OUT] llm-codex> /service-start --name pm-svc -- proc src_v1 sleep 30
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Request received. Starting service pm-svc...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.818317337Z"}
DEBUG: [REPL][OUT] DIALTONE> Request received. Starting service pm-svc...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc started as pid 979556.","subtone_pid":979556,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.8190431Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service room: service:pm-svc","subtone_pid":979556,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819048109Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service log file: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","subtone_pid":979556,"log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819049595Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc is running.","subtone_pid":979556,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819051579Z"}
DEBUG: [REPL][ROOM][repl.subtone.979556] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-979556","message":"Started at 2026-03-18T21:26:53-07:00","subtone_pid":979556,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819053385Z"}
DEBUG: [REPL][ROOM][repl.subtone.979556] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-979556","message":"Command: [proc src_v1 sleep 30]","subtone_pid":979556,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819055315Z"}
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc started as pid 979556.
DEBUG: [REPL][OUT] DIALTONE> Service room: service:pm-svc
DEBUG: [REPL][OUT] DIALTONE> Service log file: /tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc is running.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-list" expect_room=7 expect_output=7 timeout=30s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T04:26:53.819460644Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819689368Z"}
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":979556,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"running","log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","started_at":"2026-03-19T04:26:53Z","last_ok_at":"2026-03-19T04:26:53Z","service_name":"pm-svc","subtone_pid":979556}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819911153Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.819914533Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           local      979556   2026-03-19T04:26:53Z     active   service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.81991646Z"}
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> pm-svc           local      979556   2026-03-19T04:26:53Z     active   service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-stop --name pm-svc" expect_room=3 expect_output=3 timeout=30s
INFO: [REPL][INPUT] /service-stop --name pm-svc
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-stop --name pm-svc","timestamp":"2026-03-19T04:26:53.82026481Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-stop --name pm-svc","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.820600639Z"}
DEBUG: [REPL][OUT] llm-codex> /service-stop --name pm-svc
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Stopping service pm-svc (pid 979556).","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.820939583Z"}
DEBUG: [REPL][OUT] DIALTONE> Stopping service pm-svc (pid 979556).
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":979556,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"stopped","started_at":"0001-01-01T00:00:00Z","last_ok_at":"2026-03-19T04:26:53Z","exit_code":-1,"service_name":"pm-svc","subtone_pid":979556}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc stopped.","subtone_pid":979556,"exit_code":-1,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.821103828Z"}
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc stopped.
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":979556,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"stopped","log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","started_at":"2026-03-19T04:26:53Z","last_ok_at":"2026-03-19T04:26:53Z","uptime_sec":1,"exit_code":-1,"service_name":"pm-svc","subtone_pid":979556}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Stopped service pm-svc.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.921583111Z"}
DEBUG: [REPL][OUT] DIALTONE> Stopped service pm-svc.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-list" expect_room=6 expect_output=6 timeout=30s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T04:26:53.922218192Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.922636825Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.922712792Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.922730201Z"}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           DIALTONE-SERVER 979556   2026-03-19T04:26:53Z     done     service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.92273304Z"}
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> pm-svc           DIALTONE-SERVER 979556   2026-03-19T04:26:53Z     done     service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:service-start-publishes-heartbeat-and-service-registry-state] service pm-svc pid 979556 emitted heartbeats and stayed visible in service registry
INFO: report: Started named service pm-svc as pid 979556 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"validation","message":"Validation passed for service-start-publishes-heartbeat-and-service-registry-state: service pm-svc pid 979556 emitted heartbeats and stayed visible in service registry","room":"index","scope":"index","type":"line"}
PASS: [TEST][PASS] [STEP:service-start-publishes-heartbeat-and-service-registry-state] report: Started named service pm-svc as pid 979556 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ external-service-heartbeat-appears-in-service-list

- **Duration**: 4.119563ms
- **Report**: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.

#### Logs

```text
DEBUG: [REPL][OUT] DIALTONE> Validation passed for service-start-publishes-heartbeat-and-service-registry-state: service pm-svc pid 979556 emitted heartbeats and stayed visible in service registry
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"lifecycle","message":"Starting test: external-service-heartbeat-appears-in-service-list","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Starting test: external-service-heartbeat-appears-in-service-list
DEBUG: [REPL][ROOM][repl.host.legion.heartbeat.service.chrome-dev] {"command":"chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion","host":"legion","kind":"service","last_ok_at":"2026-03-19T04:26:53Z","log_path":"C:/Users/test/.dialtone/bin/dialtone_chrome_v3.out.log","mode":"service","name":"chrome-dev","pid":42424,"room":"service:chrome-dev","started_at":"2026-03-19T04:26:43Z","state":"running"}
INFO: [REPL][STEP 1] send="/service-list" expect_room=8 expect_output=7 timeout=20s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T04:26:53.926680082Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.926903421Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.926918081Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.926955836Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"chrome-dev       legion     42424    2026-03-19T04:26:53Z     active   service      chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion","log_path":"C:/Users/test/.dialtone/bin/dialtone_chrome_v3.out.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.926966262Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           DIALTONE-SERVER 979556   2026-03-19T04:26:53Z     done     service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-3639358475/repo/.dialtone/logs/subtone-979556-20260318-212653.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T04:26:53.926969773Z"}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> chrome-dev       legion     42424    2026-03-19T04:26:53Z     active   service      chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion
DEBUG: [REPL][OUT] DIALTONE> pm-svc           DIALTONE-SERVER 979556   2026-03-19T04:26:53Z     done     service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:external-service-heartbeat-appears-in-service-list] external service heartbeat for chrome-dev appeared in service-list as host legion
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"validation","message":"Validation passed for external-service-heartbeat-appears-in-service-list: external service heartbeat for chrome-dev appeared in service-list as host legion","room":"index","scope":"index","type":"line"}
INFO: report: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.
DEBUG: [REPL][OUT] DIALTONE> Validation passed for external-service-heartbeat-appears-in-service-list: external service heartbeat for chrome-dev appeared in service-list as host legion
PASS: [TEST][PASS] [STEP:external-service-heartbeat-appears-in-service-list] report: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.
```

#### Browser Logs

```text
<empty>
```

---

