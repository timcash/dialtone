# REPL Plugin src_v3 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 14 Mar 2026 16:57:00 -0700
**Version:** `repl-src-v3`
**Runner:** `test/src_v1`
**Status:** ❌ FAIL
**Total Time:** `48.021879s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| tmp-bootstrap-workspace | ✅ PASS | `176.875µs` |
| dialtone-help-surfaces | ✅ PASS | `1.587452s` |
| interactive-add-host-updates-dialtone-json | ✅ PASS | `1.255191542s` |
| interactive-help-and-ps | ✅ PASS | `558.832917ms` |
| interactive-ssh-wsl-command | ✅ PASS | `4.027087833s` |
| interactive-cloudflare-tunnel-start | ❌ FAIL | `40.593070875s` |

## Step Details

## tmp-bootstrap-workspace

### Results

```text
result: PASS
duration: 176.875µs
report: Verified ./dialtone.sh repl src_v3 test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.
```

### Logs

```text
logs:
PASS: [TEST][PASS] [STEP:tmp-bootstrap-workspace] tmp workspace is active at /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo
INFO: report: Verified ./dialtone.sh repl src_v3 test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.
PASS: [TEST][PASS] [STEP:tmp-bootstrap-workspace] report: Verified ./dialtone.sh repl src_v3 test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.
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

## dialtone-help-surfaces

### Results

```text
result: PASS
duration: 1.587452s
report: Verified help surfaces through ./dialtone.sh help and ./dialtone.sh repl src_v3 help in tmp bootstrap workspace.
```

### Logs

```text
logs:
PASS: [TEST][PASS] [STEP:dialtone-help-surfaces] verified dialtone and repl src_v3 help output
INFO: report: Verified help surfaces through ./dialtone.sh help and ./dialtone.sh repl src_v3 help in tmp bootstrap workspace.
PASS: [TEST][PASS] [STEP:dialtone-help-surfaces] report: Verified help surfaces through ./dialtone.sh help and ./dialtone.sh repl src_v3 help in tmp bootstrap workspace.
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

## interactive-add-host-updates-dialtone-json

### Results

```text
result: PASS
duration: 1.255191542s
report: Joined REPL as llm-codex, typed /repl src_v3 add-host through the live prompt path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:14.848631Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader online on DIALTONE-SERVER (subject=repl.room.index nats=nats://0.0.0.0:46222)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:14.848678Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=false count=0 (each needs name+host+user)
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T23:56:15.223589Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T23:56:15.22361Z"}
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
INFO: [REPL][STEP 1] send="/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user" expect_room=7 expect_output=8 timeout=40s
INFO: [REPL][INPUT] /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.223824Z"}
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","timestamp":"2026-03-14T23:56:15.223897Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.224013Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for repl src_v3...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for repl src_v3...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.224037Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"Started at 2026-03-14T16:56:15-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.225482Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.225503Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-40957-20260314-165615.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.225508Z"}
INFO: [REPL][OUT] DIALTONE:40957> Started at 2026-03-14T16:56:15-07:00
INFO: [REPL][OUT] DIALTONE:40957> Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]
INFO: [REPL][OUT] DIALTONE:40957> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-40957-20260314-165615.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e Verifying dependencies...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234111Z"}
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE:40957> DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e Bootstrap path checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234168Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234176Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234194Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234283Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234324Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234389Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234548Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"DIALTONE\u003e Environment ready. Launching Dialtone...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.234595Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404223Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:793] DIALTONE\u003e Bootstrap checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404244Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404256Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404286Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404304Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404321Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404331Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404342Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:805] DIALTONE\u003e State:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404347Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:807] DIALTONE\u003e - nats endpoint nats://127.0.0.1:47222 reachable=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.404404Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:808] DIALTONE\u003e - repl leader process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.430734Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:811] DIALTONE\u003e - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.441607Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.452923Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.453196Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464528Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464546Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464553Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464583Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464589Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464607Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464612Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464616Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464621Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464624Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464629Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464649Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464653Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464657Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464672Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=false count=0 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.464701Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=false count=0 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:81] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.634355Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:85] Added mesh host wsl (user@wsl.shad-artichoke.ts.net:22)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.63437Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:40957","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:87] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.634375Z"}
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:81] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:85] Added mesh host wsl (user@wsl.shad-artichoke.ts.net:22)
INFO: [REPL][OUT] DIALTONE:40957> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:87] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for repl src_v3 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.637957Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for repl src_v3 exited with code 0.
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:interactive-add-host-updates-dialtone-json] interactive add-host wrote wsl mesh node to env/dialtone.json
INFO: report: Joined REPL as llm-codex, typed /repl src_v3 add-host through the live prompt path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.
PASS: [TEST][PASS] [STEP:interactive-add-host-updates-dialtone-json] report: Joined REPL as llm-codex, typed /repl src_v3 add-host through the live prompt path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.
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

## interactive-help-and-ps

### Results

```text
result: PASS
duration: 558.832917ms
report: Joined REPL as llm-codex, typed /help and /ps through the live prompt path, and validated the room output includes the user input lines, help text, and ps empty-state response.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:15.641945Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T23:56:16.195937Z"}
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T23:56:16.195997Z"}
INFO: [REPL][STEP 1] send="/help" expect_room=5 expect_output=3 timeout=30s
INFO: [REPL][INPUT] /help
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196187Z"}
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/help","timestamp":"2026-03-14T23:56:16.196235Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/help","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196387Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /help
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Help","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196394Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196411Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Bootstrap","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196413Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`dev install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196414Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install latest Go and bootstrap dev.go command scaffold","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196415Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196423Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Plugins","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196423Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`robot src_v1 install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196424Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install robot src_v1 dependencies","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196425Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196426Z"}
INFO: [REPL][OUT] DIALTONE> Help
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> Bootstrap
INFO: [REPL][OUT] DIALTONE> `dev install`
INFO: [REPL][OUT] DIALTONE> Install latest Go and bootstrap dev.go command scaffold
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> Plugins
INFO: [REPL][OUT] DIALTONE> `robot src_v1 install`
INFO: [REPL][OUT] DIALTONE> Install robot src_v1 dependencies
INFO: [REPL][OUT] DIALTONE>
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`dag src_v3 install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196436Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install dag src_v3 dependencies","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196438Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196439Z"}
INFO: [REPL][OUT] DIALTONE> `dag src_v3 install`
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`logs src_v1 test`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.19644Z"}
INFO: [REPL][OUT] DIALTONE> Install dag src_v3 dependencies
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Run logs plugin tests on a subtone","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196441Z"}
INFO: [REPL][OUT] DIALTONE>
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196442Z"}
INFO: [REPL][OUT] DIALTONE> `logs src_v1 test`
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"System","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196442Z"}
INFO: [REPL][OUT] DIALTONE> Run logs plugin tests on a subtone
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`ps`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196443Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"List active subtones","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196444Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196445Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`kill \u003cpid\u003e`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196445Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Kill a managed subtone process by PID","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196446Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196452Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`\u003cany command\u003e`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196453Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Run any dialtone command on a managed subtone","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196454Z"}
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> System
INFO: [REPL][OUT] DIALTONE> `ps`
INFO: [REPL][OUT] DIALTONE> List active subtones
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> `kill <pid>`
INFO: [REPL][OUT] DIALTONE> Kill a managed subtone process by PID
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> `<any command>`
INFO: [REPL][OUT] DIALTONE> Run any dialtone command on a managed subtone
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ps" expect_room=4 expect_output=2 timeout=30s
INFO: [REPL][INPUT] /ps
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ps","timestamp":"2026-03-14T23:56:16.196845Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ps","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.196966Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"No active subtones.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.19701Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ps
INFO: [REPL][OUT] DIALTONE> No active subtones.
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:interactive-help-and-ps] help and ps executed through llm-codex REPL prompt path
INFO: report: Joined REPL as llm-codex, typed /help and /ps through the live prompt path, and validated the room output includes the user input lines, help text, and ps empty-state response.
PASS: [TEST][PASS] [STEP:interactive-help-and-ps] report: Joined REPL as llm-codex, typed /help and /ps through the live prompt path, and validated the room output includes the user input lines, help text, and ps empty-state response.
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

## interactive-ssh-wsl-command

### Results

```text
result: PASS
duration: 4.027087833s
report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus a usable auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the REPL subtone returned the remote user output.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.201322Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T23:56:16.764136Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T23:56:16.764179Z"}
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
INFO: [REPL][STEP 1] send="/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user" expect_room=6 expect_output=7 timeout=35s
INFO: [REPL][INPUT] /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.764363Z"}
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","timestamp":"2026-03-14T23:56:16.764422Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.764615Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for repl src_v3...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.764627Z"}
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for repl src_v3...
INFO: [REPL][OUT] DIALTONE:41058> Started at 2026-03-14T16:56:16-07:00
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"Started at 2026-03-14T16:56:16-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.766096Z"}
INFO: [REPL][OUT] DIALTONE:41058> Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]
INFO: [REPL][OUT] DIALTONE:41058> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41058-20260314-165616.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.766126Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41058-20260314-165616.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.766129Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e Verifying dependencies...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771528Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e Bootstrap path checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771546Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.77157Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771584Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771591Z"}
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> Bootstrap path checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771628Z"}
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771673Z"}
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771828Z"}
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"DIALTONE\u003e Environment ready. Launching Dialtone...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.771906Z"}
INFO: [REPL][OUT] DIALTONE:41058> DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.928964Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:793] DIALTONE\u003e Bootstrap checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.928979Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929014Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929024Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929048Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929063Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929079Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929088Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:805] DIALTONE\u003e State:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.929096Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:807] DIALTONE\u003e - nats endpoint nats://127.0.0.1:47222 reachable=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.92925Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:808] DIALTONE\u003e - repl leader process running=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.943225Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:811] DIALTONE\u003e - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.953999Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.964502Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.964727Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.9754Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975418Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975424Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975431Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975436Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975479Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975488Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975502Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975507Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975517Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975531Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975536Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.97554Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975543Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975548Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:16.975552Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:81] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.129798Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:83] Updated mesh host wsl (user@wsl.shad-artichoke.ts.net:22)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.129811Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41058","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:87] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.129817Z"}
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:81] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:83] Updated mesh host wsl (user@wsl.shad-artichoke.ts.net:22)
INFO: [REPL][OUT] DIALTONE:41058> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:87] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for repl src_v3 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.133869Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for repl src_v3 exited with code 0.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ssh src_v1 resolve --host wsl" expect_room=6 expect_output=7 timeout=35s
INFO: [REPL][INPUT] /ssh src_v1 resolve --host wsl
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ssh src_v1 resolve --host wsl","timestamp":"2026-03-14T23:56:17.134143Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ssh src_v1 resolve --host wsl","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.134256Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ssh src_v1 resolve --host wsl
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for ssh src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.134273Z"}
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for ssh src_v1...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"Started at 2026-03-14T16:56:17-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.135429Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"Command: [ssh src_v1 resolve --host wsl]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.135443Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41078-20260314-165617.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.135445Z"}
INFO: [REPL][OUT] DIALTONE:41078> Started at 2026-03-14T16:56:17-07:00
INFO: [REPL][OUT] DIALTONE:41078> Command: [ssh src_v1 resolve --host wsl]
INFO: [REPL][OUT] DIALTONE:41078> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41078-20260314-165617.log
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> Bootstrap path checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e Verifying dependencies...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.140818Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e Bootstrap path checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.14083Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.140862Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.140879Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.140931Z"}
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.14098Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.140992Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.141102Z"}
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"DIALTONE\u003e Environment ready. Launching Dialtone...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.14128Z"}
INFO: [REPL][OUT] DIALTONE:41078> DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.326934Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:793] DIALTONE\u003e Bootstrap checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.32695Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.326956Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.326963Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.326968Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.326994Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.327007Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.327023Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:805] DIALTONE\u003e State:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.327029Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:807] DIALTONE\u003e - nats endpoint nats://127.0.0.1:47222 reachable=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.327075Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:808] DIALTONE\u003e - repl leader process running=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.338292Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:811] DIALTONE\u003e - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.348513Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.358901Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.359186Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.369964Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.369978Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370009Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370026Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370033Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370051Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370057Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370066Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370071Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370081Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370086Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.37009Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370093Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370097Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370101Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:17.370129Z"}
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] DIALTONE:41078> [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"name=wsl","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417537Z"}
INFO: [REPL][OUT] DIALTONE:41078> name=wsl
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"transport=ssh","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417603Z"}
INFO: [REPL][OUT] DIALTONE:41078> transport=ssh
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"user=user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417634Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"port=22","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417666Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"preferred=wsl.shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417693Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"auth=inline-private-key","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417719Z"}
INFO: [REPL][OUT] DIALTONE:41078> user=user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"host_key=insecure-ignore","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417769Z"}
INFO: [REPL][OUT] DIALTONE:41078> port=22
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"route.tailscale=wsl.shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417802Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"route.private=","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417906Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41078","message":"candidates=wsl.shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.417969Z"}
INFO: [REPL][OUT] DIALTONE:41078> preferred=wsl.shad-artichoke.ts.net
INFO: [REPL][OUT] DIALTONE:41078> auth=inline-private-key
INFO: [REPL][OUT] DIALTONE:41078> host_key=insecure-ignore
INFO: [REPL][OUT] DIALTONE:41078> route.tailscale=wsl.shad-artichoke.ts.net
INFO: [REPL][OUT] DIALTONE:41078> route.private=
INFO: [REPL][OUT] DIALTONE:41078> candidates=wsl.shad-artichoke.ts.net
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for ssh src_v1 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.432665Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for ssh src_v1 exited with code 0.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ssh src_v1 run --host wsl --cmd whoami" expect_room=7 expect_output=8 timeout=1m30s
INFO: [REPL][INPUT] /ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ssh src_v1 run --host wsl --cmd whoami","timestamp":"2026-03-14T23:56:18.631547Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.631984Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for ssh src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.632039Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ssh src_v1 run --host wsl --cmd whoami
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for ssh src_v1...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"Started at 2026-03-14T16:56:18-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.636962Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"Command: [ssh src_v1 run --host wsl --cmd whoami]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.637012Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41102-20260314-165618.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.637018Z"}
INFO: [REPL][OUT] DIALTONE:41102> Started at 2026-03-14T16:56:18-07:00
INFO: [REPL][OUT] DIALTONE:41102> Command: [ssh src_v1 run --host wsl --cmd whoami]
INFO: [REPL][OUT] DIALTONE:41102> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41102-20260314-165618.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e Verifying dependencies...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652345Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e Bootstrap path checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652388Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652469Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652505Z"}
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.65271Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652762Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652913Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.652999Z"}
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"DIALTONE\u003e Environment ready. Launching Dialtone...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.65325Z"}
INFO: [REPL][OUT] DIALTONE:41102> DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865104Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:793] DIALTONE\u003e Bootstrap checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865119Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865159Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865183Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865191Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865201Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865206Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865214Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:805] DIALTONE\u003e State:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.86526Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:807] DIALTONE\u003e - nats endpoint nats://127.0.0.1:47222 reachable=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.865338Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:808] DIALTONE\u003e - repl leader process running=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.876261Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:811] DIALTONE\u003e - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.887596Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.898093Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.898335Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909009Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909025Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909061Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909076Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909083Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909097Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909103Z"}
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] DIALTONE:41102> [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909107Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909111Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909115Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.90917Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909175Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909181Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909186Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.90919Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:18.909195Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:19.8499Z"}
INFO: [REPL][OUT] DIALTONE:41102> user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41102","message":"user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.217076Z"}
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:interactive-ssh-wsl-command] ssh wsl command routed through llm-codex REPL prompt path
INFO: report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus a usable auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the REPL subtone returned the remote user output.
PASS: [TEST][PASS] [STEP:interactive-ssh-wsl-command] report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus a usable auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the REPL subtone returned the remote user output.
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

## interactive-cloudflare-tunnel-start

### Results

```text
result: FAIL
duration: 40.593070875s
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.233799Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T23:56:20.813261Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T23:56:20.813284Z"}
INFO: [REPL][STEP 1] send="/cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080" expect_room=6 expect_output=7 timeout=40s
INFO: [REPL][INPUT] /cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.813444Z"}
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080","timestamp":"2026-03-14T23:56:20.813552Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.813659Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for cloudflare src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.813672Z"}
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for cloudflare src_v1...
INFO: [REPL][OUT] DIALTONE:41162> Started at 2026-03-14T16:56:20-07:00
INFO: [REPL][OUT] DIALTONE:41162> Command: [cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080]
INFO: [REPL][OUT] DIALTONE:41162> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41162-20260314-165620.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"Started at 2026-03-14T16:56:20-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.814995Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"Command: [cloudflare src_v1 tunnel start repl-src-v3-test --url http://127.0.0.1:8080]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.815025Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/.dialtone/logs/subtone-41162-20260314-165620.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.815027Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e Verifying dependencies...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820311Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e Bootstrap path checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820324Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820353Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820369Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.82039Z"}
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE:41162> DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.82042Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.82048Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820598Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"DIALTONE\u003e Environment ready. Launching Dialtone...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.820679Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970271Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:793] DIALTONE\u003e Bootstrap checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970313Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970321Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970326Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970333Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970338Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970343Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:862] DIALTONE\u003e - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970347Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:805] DIALTONE\u003e State:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970352Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:807] DIALTONE\u003e - nats endpoint nats://127.0.0.1:47222 reachable=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.970362Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:182] [CONFIG] Loaded from /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:793] DIALTONE> Bootstrap checks:
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo (dir)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/src (dir)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - env json: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - mesh config: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:862] DIALTONE> - go bin: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-177787320/dialtone_env/go/bin/go (file)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:805] DIALTONE> State:
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:807] DIALTONE> - nats endpoint nats://127.0.0.1:47222 reachable=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:808] DIALTONE\u003e - repl leader process running=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.982305Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:808] DIALTONE> - repl leader process running=true
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:811] DIALTONE\u003e - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:20.992934Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:811] DIALTONE> - tailnet active=true provider=localapi tailnet=shad-artichoke.ts.net
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003549Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:816] DIALTONE> - cloudflare running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003824Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:829] DIALTONE> - bootstrap http http://127.0.0.1:8811/install.sh running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014608Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:830] DIALTONE> - bootstrap http process running=false
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014623Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:835] DIALTONE> Command checks:
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.01466Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014676Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014681Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014701Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014706Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014714Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014718Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014722Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014725Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014739Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014783Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014791Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014797Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014809Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:836] DIALTONE> - help command available=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:837] DIALTONE> - ps command available=true (proc scaffold)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:838] DIALTONE> - ssh command available=true (ssh scaffold)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:839] DIALTONE> - repl command path available=true (repl scaffold)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:840] DIALTONE> - repl injection ready=false
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:841] DIALTONE> - repl autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:842] DIALTONE> - bootstrap http autostart enabled=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:910] DIALTONE> env/dialtone.json checks:
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:925] DIALTONE> - format valid=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_ENV present=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:929] DIALTONE> - required DIALTONE_REPO_ROOT present=true
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:933] DIALTONE> - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:936] DIALTONE> - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|INFO|src/dev.go:939] DIALTONE> - mesh_nodes valid=true count=1 (each needs name+host+user)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|ERROR|src/plugins/cloudflare/scaffold/main.go:35] cloudflare error: exec: \"cloudflared\": executable file not found in $PATH","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.605774Z"}
INFO: [REPL][OUT] DIALTONE:41162> [T+0000s|ERROR|src/plugins/cloudflare/scaffold/main.go:35] cloudflare error: exec: "cloudflared": executable file not found in $PATH
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.606255Z"}
INFO: [REPL][OUT] DIALTONE:41162> [ERROR] exit status 1
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.60982Z"}
INFO: [REPL][OUT] DIALTONE:41162> [ERROR] exit status 1
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for cloudflare src_v1 exited with code 1.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.611925Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for cloudflare src_v1 exited with code 1.
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:24.84979Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:29.849636Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:34.849548Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:39.848508Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:44.84931Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:49.849175Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:54.849077Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:59.848551Z"}
```

### Errors

```text
errors:
ERROR: timeout waiting for room patterns: Subtone for cloudflare src_v1 exited with code 0.
recent room messages:
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003549Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003824Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014608Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014623Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.01466Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014676Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014681Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014701Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014706Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014714Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014718Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014722Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014725Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014739Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014783Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014791Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014797Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014809Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|ERROR|src/plugins/cloudflare/scaffold/main.go:35] cloudflare error: exec: \"cloudflared\": executable file not found in $PATH","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.605774Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.606255Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.60982Z"}
{"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for cloudflare src_v1 exited with code 1.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.611925Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:24.84979Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:29.849636Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:34.849548Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:39.848508Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:44.84931Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:49.849175Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:54.849077Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:59.848551Z"}
FAIL: [TEST][FAIL] [STEP:interactive-cloudflare-tunnel-start] failed: transcript step 1 room expect failed: timeout waiting for room patterns: Subtone for cloudflare src_v1 exited with code 0.
recent room messages:
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:816] DIALTONE\u003e - cloudflare running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003549Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:829] DIALTONE\u003e - bootstrap http http://127.0.0.1:8811/install.sh running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.003824Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:830] DIALTONE\u003e - bootstrap http process running=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014608Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:835] DIALTONE\u003e Command checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014623Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:836] DIALTONE\u003e - help command available=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.01466Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:837] DIALTONE\u003e - ps command available=true (proc scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014676Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:838] DIALTONE\u003e - ssh command available=true (ssh scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014681Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:839] DIALTONE\u003e - repl command path available=true (repl scaffold)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014701Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:840] DIALTONE\u003e - repl injection ready=false","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014706Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:841] DIALTONE\u003e - repl autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014714Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:842] DIALTONE\u003e - bootstrap http autostart enabled=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014718Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:910] DIALTONE\u003e env/dialtone.json checks:","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014722Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:925] DIALTONE\u003e - format valid=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014725Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_ENV present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014739Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:929] DIALTONE\u003e - required DIALTONE_REPO_ROOT present=true","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014783Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:933] DIALTONE\u003e - tsnet bootstrap keys present=false (TS_AUTHKEY or TS_API_KEY+TS_TAILNET)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014791Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:936] DIALTONE\u003e - cloudflare bootstrap keys present=false (CF_TUNNEL_TOKEN_SHELL or CLOUDFLARE_API_TOKEN+CLOUDFLARE_ACCOUNT_ID)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014797Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|INFO|src/dev.go:939] DIALTONE\u003e - mesh_nodes valid=true count=1 (each needs name+host+user)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:21.014809Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[T+0000s|ERROR|src/plugins/cloudflare/scaffold/main.go:35] cloudflare error: exec: \"cloudflared\": executable file not found in $PATH","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.605774Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.606255Z"}
{"type":"line","room":"index","prefix":"DIALTONE:41162","message":"[ERROR] exit status 1","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.60982Z"}
{"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for cloudflare src_v1 exited with code 1.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:22.611925Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:24.84979Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:29.849636Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:34.849548Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:39.848508Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:44.84931Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:49.849175Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:54.849077Z"}
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:59.848551Z"}
```

### Browser Logs

```text
browser_logs:
<empty>
```

