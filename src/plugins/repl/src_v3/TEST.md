# REPL Plugin src_v3 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Wed, 18 Mar 2026 17:05:53 -0700
**Version:** `repl-src-v3`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `3.437996905s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| service-start-publishes-heartbeat-and-service-registry-state | ✅ PASS | `3.435583335s` |
| external-service-heartbeat-appears-in-service-list | ✅ PASS | `2.395397ms` |

## Step Details

## service-start-publishes-heartbeat-and-service-registry-state

### Results

```text
result: PASS
duration: 3.435583335s
report: Started named service pm-svc as pid 873719 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.270191851Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.270197795Z"}
DEBUG: [REPL][OUT] DIALTONE> Verifying dependencies...
DEBUG: [REPL][OUT] DIALTONE> Bootstrap path checks:
DEBUG: [REPL][OUT] DIALTONE> - repo root: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo (dir)
DEBUG: [REPL][OUT] DIALTONE> - src root: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/src (dir)
DEBUG: [REPL][OUT] DIALTONE> - env dir: /tmp/dialtone-repl-v3-bootstrap-2589274204/dialtone_env (dir)
DEBUG: [REPL][OUT] DIALTONE> - env json: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json (file)
DEBUG: [REPL][OUT] DIALTONE> - mesh config: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json (file)
DEBUG: [REPL][OUT] DIALTONE> Using managed Go (Cached): /tmp/dialtone-repl-v3-bootstrap-2589274204/dialtone_env/go/bin/go
DEBUG: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-19T00:05:53.687268287Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","timestamp":"2026-03-19T00:05:53.687360162Z"}
DEBUG: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
DEBUG: [REPL][OUT] DIALTONE> llm-codex joined room index (version=dev).
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"lifecycle","message":"Starting test: service-start-publishes-heartbeat-and-service-registry-state","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.687694582Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.687711222Z"}
DEBUG: [REPL][OUT] DIALTONE> Starting test: service-start-publishes-heartbeat-and-service-registry-state
DEBUG: [REPL][OUT] DIALTONE> Leader active on DIALTONE-SERVER
DEBUG: [REPL][OUT] DIALTONE> Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)
DEBUG: [REPL][OUT] DIALTONE> Shared REPL session ready for llm-codex in room index.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Shared REPL session ready for llm-codex in room index.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Checking required files.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Checking required files.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"repo root: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo (dir)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> repo root: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo (dir)
DEBUG: [REPL][OUT] DIALTONE> src/dev.go: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/src/dev.go (file)
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"src/dev.go: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/src/dev.go (file)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"env/dialtone.json: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json (file)","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> env/dialtone.json: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json (file)
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"Checking runtime variables.","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Checking runtime variables.
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_REPO_ROOT: set=true value=/tmp/dialtone-repl-v3-bootstrap-2589274204/repo","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_REPO_ROOT: set=true value=/tmp/dialtone-repl-v3-bootstrap-2589274204/repo
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_ENV_FILE: set=true value=/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_ENV_FILE: set=true value=/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/env/dialtone.json
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"DIALTONE_REPL_NATS_URL: set=true value=nats://127.0.0.1:46222","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> DIALTONE_REPL_NATS_URL: set=true value=nats://127.0.0.1:46222
DEBUG: [REPL][OUT] DIALTONE> dialtone.json metadata: mesh_nodes=0 names=none tailscale_keys=false cloudflare_keys=false
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"status","message":"dialtone.json metadata: mesh_nodes=0 names=none tailscale_keys=false cloudflare_keys=false","room":"index","scope":"index","type":"line"}
INFO: [REPL][STEP 1] send="/service-start --name pm-svc -- proc src_v1 sleep 30" expect_room=6 expect_output=6 timeout=30s
INFO: [REPL][INPUT] /service-start --name pm-svc -- proc src_v1 sleep 30
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-start --name pm-svc -- proc src_v1 sleep 30","timestamp":"2026-03-19T00:05:53.690733523Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-start --name pm-svc -- proc src_v1 sleep 30","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.691080242Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Request received. Starting service pm-svc...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.691158372Z"}
DEBUG: [REPL][OUT] llm-codex> /service-start --name pm-svc -- proc src_v1 sleep 30
DEBUG: [REPL][OUT] DIALTONE> Request received. Starting service pm-svc...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc started as pid 873719.","subtone_pid":873719,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692066227Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service room: service:pm-svc","subtone_pid":873719,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692071577Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service log file: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","subtone_pid":873719,"log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692073216Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc is running.","subtone_pid":873719,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692078546Z"}
DEBUG: [REPL][ROOM][repl.subtone.873719] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-873719","message":"Started at 2026-03-18T17:05:53-07:00","subtone_pid":873719,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692080631Z"}
DEBUG: [REPL][ROOM][repl.subtone.873719] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-873719","message":"Command: [proc src_v1 sleep 30]","subtone_pid":873719,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.692082703Z"}
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc started as pid 873719.
DEBUG: [REPL][OUT] DIALTONE> Service room: service:pm-svc
DEBUG: [REPL][OUT] DIALTONE> Service log file: /tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc is running.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-list" expect_room=7 expect_output=7 timeout=30s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T00:05:53.692836852Z"}
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":873719,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"running","log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","started_at":"2026-03-19T00:05:53Z","last_ok_at":"2026-03-19T00:05:53Z","service_name":"pm-svc","subtone_pid":873719}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.693091887Z"}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.693347926Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.693351831Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     active   service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.693354068Z"}
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     active   service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-stop --name pm-svc" expect_room=3 expect_output=3 timeout=30s
INFO: [REPL][INPUT] /service-stop --name pm-svc
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-stop --name pm-svc","timestamp":"2026-03-19T00:05:53.693937027Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-stop --name pm-svc","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.694295188Z"}
DEBUG: [REPL][OUT] llm-codex> /service-stop --name pm-svc
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Stopping service pm-svc (pid 873719).","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.694515547Z"}
DEBUG: [REPL][OUT] DIALTONE> Stopping service pm-svc (pid 873719).
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":873719,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"stopped","started_at":"0001-01-01T00:00:00Z","last_ok_at":"2026-03-19T00:05:53Z","exit_code":-1,"service_name":"pm-svc","subtone_pid":873719}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Service pm-svc stopped.","subtone_pid":873719,"exit_code":-1,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.69476451Z"}
DEBUG: [REPL][OUT] DIALTONE> Service pm-svc stopped.
DEBUG: [REPL][ROOM][repl.host.dialtone-server.heartbeat.service.pm-svc] {"host":"DIALTONE-SERVER","kind":"service","name":"pm-svc","mode":"service","pid":873719,"room":"service:pm-svc","command":"proc src_v1 sleep 30","state":"stopped","log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","started_at":"2026-03-19T00:05:53Z","last_ok_at":"2026-03-19T00:05:53Z","uptime_sec":1,"exit_code":-1,"service_name":"pm-svc","subtone_pid":873719}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Stopped service pm-svc.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.79574051Z"}
DEBUG: [REPL][OUT] DIALTONE> Stopped service pm-svc.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/service-list" expect_room=6 expect_output=6 timeout=30s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T00:05:53.796459697Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.796721305Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.796741762Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.796744911Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     done     service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.796747293Z"}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     done     service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:service-start-publishes-heartbeat-and-service-registry-state] service pm-svc pid 873719 emitted heartbeats and stayed visible in service registry
INFO: report: Started named service pm-svc as pid 873719 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.
PASS: [TEST][PASS] [STEP:service-start-publishes-heartbeat-and-service-registry-state] report: Started named service pm-svc as pid 873719 through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name pm-svc`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
<empty>
```

## external-service-heartbeat-appears-in-service-list

### Results

```text
result: PASS
duration: 2.395397ms
report: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.room.index] {"kind":"lifecycle","message":"Starting test: external-service-heartbeat-appears-in-service-list","room":"index","scope":"index","type":"line"}
DEBUG: [REPL][OUT] DIALTONE> Starting test: external-service-heartbeat-appears-in-service-list
DEBUG: [REPL][ROOM][repl.host.legion.heartbeat.service.chrome-dev] {"command":"chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion","host":"legion","kind":"service","last_ok_at":"2026-03-19T00:05:53Z","log_path":"C:/Users/test/.dialtone/bin/dialtone_chrome_v3.out.log","mode":"service","name":"chrome-dev","pid":42424,"room":"service:chrome-dev","started_at":"2026-03-19T00:05:43Z","state":"running"}
INFO: [REPL][STEP 1] send="/service-list" expect_room=8 expect_output=7 timeout=20s
INFO: [REPL][INPUT] /service-list
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"linux","arch":"amd64","message":"/service-list","timestamp":"2026-03-19T00:05:53.798890143Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/service-list","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.799147654Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"Managed Services:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.799179865Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.799191521Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"chrome-dev       legion     42424    2026-03-19T00:05:53Z     active   service      chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion","log_path":"C:/Users/test/.dialtone/bin/dialtone_chrome_v3.out.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.799193834Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"status","room":"index","message":"pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     done     service      proc src_v1 sleep 30","log_path":"/tmp/dialtone-repl-v3-bootstrap-2589274204/repo/.dialtone/logs/subtone-873719-20260318-170553.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-19T00:05:53.799195625Z"}
DEBUG: [REPL][OUT] llm-codex> /service-list
DEBUG: [REPL][OUT] DIALTONE> Managed Services:
DEBUG: [REPL][OUT] DIALTONE> NAME             HOST       PID      UPDATED                  STATE    MODE         COMMAND
DEBUG: [REPL][OUT] DIALTONE> chrome-dev       legion     42424    2026-03-19T00:05:53Z     active   service      chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion
DEBUG: [REPL][OUT] DIALTONE> pm-svc           DIALTONE-SERVER 873719   2026-03-19T00:05:53Z     done     service      proc src_v1 sleep 30
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:external-service-heartbeat-appears-in-service-list] external service heartbeat for chrome-dev appeared in service-list as host legion
INFO: report: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.
PASS: [TEST][PASS] [STEP:external-service-heartbeat-appears-in-service-list] report: Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.
```

### Errors

```text
errors:
<empty>
```

### Browser Logs

```text
browser_logs:
<empty>
```

