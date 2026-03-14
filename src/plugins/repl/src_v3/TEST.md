# REPL Plugin src_v3 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Sat, 14 Mar 2026 15:29:11 -0700
**Version:** `repl-src-v3`
**Runner:** `test/src_v1`
**Status:** ❌ FAIL
**Total Time:** `8.626195417s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| tmp-bootstrap-workspace | ✅ PASS | `168.917µs` |
| dialtone-help-surfaces | ✅ PASS | `1.85932575s` |
| interactive-add-host-updates-dialtone-json | ✅ PASS | `1.227911167s` |
| interactive-help-and-ps | ✅ PASS | `576.800625ms` |
| interactive-ssh-wsl-command | ✅ PASS | `3.213830708s` |
| interactive-cloudflare-tunnel-start | ❌ FAIL | `1.748088833s` |

## Step Details

## tmp-bootstrap-workspace

### Results

```text
result: PASS
duration: 168.917µs
report: Verified ./dialtone.sh repl src_v3 test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.
```

### Logs

```text
logs:
PASS: [TEST][PASS] [STEP:tmp-bootstrap-workspace] tmp workspace is active at /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo
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
duration: 1.85932575s
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
duration: 1.227911167s
report: Joined REPL as llm-codex, typed /repl src_v3 add-host through the live prompt path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.515219Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader online on DIALTONE-SERVER (subject=repl.room.index nats=nats://0.0.0.0:46222)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.515269Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T22:29:05.884634Z"}
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T22:29:05.884696Z"}
INFO: [REPL][STEP 1] send="/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user" expect_room=7 expect_output=8 timeout=40s
INFO: [REPL][INPUT] /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.884923Z"}
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","timestamp":"2026-03-14T22:29:05.885015Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.885195Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for repl src_v3...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.885212Z"}
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for repl src_v3...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"Started at 2026-03-14T15:29:05-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.886871Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.886895Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36811-20260314-152905.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:05.886897Z"}
INFO: [REPL][OUT] DIALTONE:36811> Started at 2026-03-14T15:29:05-07:00
INFO: [REPL][OUT] DIALTONE:36811> Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]
INFO: [REPL][OUT] DIALTONE:36811> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36811-20260314-152905.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:80] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.27783Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:84] Added mesh host wsl (user@wsl.shad-artichoke.ts.net:22)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.277867Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36811","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:86] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.277877Z"}
INFO: [REPL][OUT] DIALTONE:36811> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:80] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:36811> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:84] Added mesh host wsl (user@wsl.shad-artichoke.ts.net:22)
INFO: [REPL][OUT] DIALTONE:36811> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:86] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for repl src_v3 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.281592Z"}
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
duration: 576.800625ms
report: Joined REPL as llm-codex, typed /help and /ps through the live prompt path, and validated the room output includes the user input lines, help text, and ps empty-state response.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.286379Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T22:29:06.858033Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T22:29:06.858064Z"}
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][STEP 1] send="/help" expect_room=5 expect_output=3 timeout=30s
INFO: [REPL][INPUT] /help
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858226Z"}
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/help","timestamp":"2026-03-14T22:29:06.858319Z"}
INFO: [REPL][OUT] llm-codex> DIALTONE> DIALTONE leader active on DIALTONE-SERVER
INFO: [REPL][OUT] llm-codex> /help
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
INFO: [REPL][OUT] DIALTONE> `dag src_v3 install`
INFO: [REPL][OUT] DIALTONE> Install dag src_v3 dependencies
INFO: [REPL][OUT] DIALTONE>
INFO: [REPL][OUT] DIALTONE> `logs src_v1 test`
INFO: [REPL][OUT] DIALTONE> Run logs plugin tests on a subtone
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
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/help","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858433Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Help","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858443Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.85845Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Bootstrap","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858451Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`dev install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858453Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install latest Go and bootstrap dev.go command scaffold","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858454Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858459Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Plugins","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858459Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`robot src_v1 install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.85846Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install robot src_v1 dependencies","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.85847Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858471Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`dag src_v3 install`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858472Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Install dag src_v3 dependencies","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858473Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858478Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`logs src_v1 test`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858479Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Run logs plugin tests on a subtone","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.85848Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858481Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"System","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858482Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`ps`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858483Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"List active subtones","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858484Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858492Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`kill \u003cpid\u003e`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858493Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Kill a managed subtone process by PID","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858494Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858504Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"`\u003cany command\u003e`","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858505Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Run any dialtone command on a managed subtone","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.858506Z"}
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ps" expect_room=4 expect_output=2 timeout=30s
INFO: [REPL][INPUT] /ps
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ps","timestamp":"2026-03-14T22:29:06.859002Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ps
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ps","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.859192Z"}
INFO: [REPL][OUT] DIALTONE> No active subtones.
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"No active subtones.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.859203Z"}
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
duration: 3.213830708s
report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the subtone lifecycle output for the SSH execution path.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:06.863415Z"}
INFO: [REPL][OUT] DIALTONE> Verifying dependencies...
INFO: [REPL][OUT] DIALTONE> Bootstrap path checks:
INFO: [REPL][OUT] DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo (dir)
INFO: [REPL][OUT] DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/src (dir)
INFO: [REPL][OUT] DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env (dir)
INFO: [REPL][OUT] DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
INFO: [REPL][OUT] DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env/go/bin/go
INFO: [REPL][OUT] DIALTONE> Environment ready. Launching Dialtone...
DEBUG: [REPL][ROOM][repl.cmd] {"type":"probe","from":"llm-codex","room":"index","message":"probe","timestamp":"2026-03-14T22:29:07.427913Z"}
INFO: [REPL][OUT] DIALTONE> Connected to repl.room.index via nats://127.0.0.1:46222
INFO: [REPL][OUT] llm-codex> DIALTONE> [JOIN] llm-codex (room=index version=dev)
DEBUG: [REPL][ROOM][repl.room.index] {"type":"join","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","timestamp":"2026-03-14T22:29:07.427954Z"}
INFO: [REPL][STEP 1] send="/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user" expect_room=6 expect_output=7 timeout=35s
INFO: [REPL][INPUT] /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"DIALTONE leader active on DIALTONE-SERVER","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.428086Z"}
INFO: [REPL][OUT] DIALTONE> DIALTONE leader active on DIALTONE-SERVER
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","timestamp":"2026-03-14T22:29:07.428199Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.428307Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for repl src_v3...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.428315Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for repl src_v3...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"Started at 2026-03-14T15:29:07-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.430122Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.430138Z"}
INFO: [REPL][OUT] DIALTONE:36921> Started at 2026-03-14T15:29:07-07:00
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36921-20260314-152907.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.43014Z"}
INFO: [REPL][OUT] DIALTONE:36921> Command: [repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user]
INFO: [REPL][OUT] DIALTONE:36921> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36921-20260314-152907.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:80] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.804401Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:82] Updated mesh host wsl (user@wsl.shad-artichoke.ts.net:22)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.804446Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36921","message":"[T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:86] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.804457Z"}
INFO: [REPL][OUT] DIALTONE:36921> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:80] Verified mesh host wsl persisted to /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json
INFO: [REPL][OUT] DIALTONE:36921> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:82] Updated mesh host wsl (user@wsl.shad-artichoke.ts.net:22)
INFO: [REPL][OUT] DIALTONE:36921> [T+0000s|INFO|src/plugins/repl/src_v3/go/repl/inject_hosts_v3.go:86] You can now run: ./dialtone.sh ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for repl src_v3 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.809036Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for repl src_v3 exited with code 0.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ssh src_v1 resolve --host wsl" expect_room=6 expect_output=7 timeout=35s
INFO: [REPL][INPUT] /ssh src_v1 resolve --host wsl
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ssh src_v1 resolve --host wsl","timestamp":"2026-03-14T22:29:07.809283Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ssh src_v1 resolve --host wsl","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.809401Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for ssh src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.809413Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ssh src_v1 resolve --host wsl
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for ssh src_v1...
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36943","message":"Started at 2026-03-14T15:29:07-07:00","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.810632Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36943","message":"Command: [ssh src_v1 resolve --host wsl]","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.810644Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE:36943","message":"Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36943-20260314-152907.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:07.810645Z"}
INFO: [REPL][OUT] DIALTONE:36943> Started at 2026-03-14T15:29:07-07:00
INFO: [REPL][OUT] DIALTONE:36943> Command: [ssh src_v1 resolve --host wsl]
INFO: [REPL][OUT] DIALTONE:36943> Log: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/.dialtone/logs/subtone-36943-20260314-152907.log
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Subtone for ssh src_v1 exited with code 0.","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:09.877951Z"}
INFO: [REPL][OUT] DIALTONE> Subtone for ssh src_v1 exited with code 0.
INFO: [REPL][STEP 1] complete
INFO: [REPL][STEP 1] send="/ssh src_v1 run --host wsl --cmd whoami" expect_room=6 expect_output=7 timeout=1m30s
INFO: [REPL][INPUT] /ssh src_v1 run --host wsl --cmd whoami
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"llm-codex","room":"index","version":"dev","os":"darwin","arch":"arm64","message":"/ssh src_v1 run --host wsl --cmd whoami","timestamp":"2026-03-14T22:29:10.066721Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"llm-codex","room":"index","message":"/ssh src_v1 run --host wsl --cmd whoami","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:10.067091Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","room":"index","prefix":"DIALTONE","message":"Request received. Spawning subtone for ssh src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T22:29:10.06719Z"}
INFO: [REPL][OUT] llm-codex> llm-codex> /ssh src_v1 run --host wsl --cmd whoami
INFO: [REPL][OUT] DIALTONE> Request received. Spawning subtone for ssh src_v1...
INFO: [REPL][STEP 1] complete
PASS: [TEST][PASS] [STEP:interactive-ssh-wsl-command] ssh wsl command routed through llm-codex REPL prompt path
INFO: report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the subtone lifecycle output for the SSH execution path.
PASS: [TEST][PASS] [STEP:interactive-ssh-wsl-command] report: Joined REPL as llm-codex, added the sample wsl host via /repl src_v3 add-host, exercised /ssh src_v1 resolve --host wsl through the prompt, verified the SSH resolve report selected the expected host/user/port plus auth source and host-key mode, then typed /ssh src_v1 run --host wsl --cmd whoami and verified the subtone lifecycle output for the SSH execution path.
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
duration: 1.748088833s
```

### Errors

```text
errors:
FAIL: [TEST][FAIL] [STEP:interactive-cloudflare-tunnel-start] failed: direct tunnel start did not execute cloudflared as expected:
DIALTONE> Verifying dependencies...
DIALTONE> Bootstrap path checks:
DIALTONE> - repo root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo (dir)
DIALTONE> - src root: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/src (dir)
DIALTONE> - env dir: /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env (dir)
DIALTONE> - env json: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
DIALTONE> - mesh config: /private/var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/repo/env/dialtone.json (file)
DIALTONE> Using managed Go (Cached): /var/folders/97/5lzp131s2g903bfnbvf1hwq00000gn/T/dialtone-repl-v3-bootstrap-851208628/dialtone_env/go/bin/go
DIALTONE> Environment ready. Launching Dialtone...
```

### Browser Logs

```text
browser_logs:
<empty>
```

