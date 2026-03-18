# REPL Plugin src_v3 Test Report

## Test Environment

```text
<empty>
```

**Generated at:** Wed, 18 Mar 2026 16:06:02 -0700
**Version:** `repl-src-v3`
**Runner:** `test/src_v1`
**Status:** ✅ PASS
**Total Time:** `5.893142779s`

## Test Steps

| Step | Result | Duration |
|---|---|---|
| shell-routed-command-autostarts-leader-when-missing | ✅ PASS | `2.717443692s` |
| shell-routed-command-reuses-running-leader | ✅ PASS | `3.175667586s` |

## Step Details

## shell-routed-command-autostarts-leader-when-missing

### Results

```text
result: PASS
duration: 2.717443692s
report: Ran `./dialtone.sh proc src_v1 emit shell-autostart-ok` against a fresh local NATS URL, verified the shell path produced the normal routed subtone lifecycle, wrote leader pid 708153 to `leader.json`, and kept the emitted payload in the subtone log instead of leaking it into shell/index output.
```

### Logs

```text
logs:
PASS: [TEST][PASS] [STEP:shell-routed-command-autostarts-leader-when-missing] shell routed command autostarted leader pid 708153 and kept payload in subtone log
INFO: report: Ran `./dialtone.sh proc src_v1 emit shell-autostart-ok` against a fresh local NATS URL, verified the shell path produced the normal routed subtone lifecycle, wrote leader pid 708153 to `leader.json`, and kept the emitted payload in the subtone log instead of leaking it into shell/index output.
PASS: [TEST][PASS] [STEP:shell-routed-command-autostarts-leader-when-missing] report: Ran `./dialtone.sh proc src_v1 emit shell-autostart-ok` against a fresh local NATS URL, verified the shell path produced the normal routed subtone lifecycle, wrote leader pid 708153 to `leader.json`, and kept the emitted payload in the subtone log instead of leaking it into shell/index output.
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

## shell-routed-command-reuses-running-leader

### Results

```text
result: PASS
duration: 3.175667586s
report: Started the REPL leader first, then ran `./dialtone.sh proc src_v1 emit shell-reuse-ok` and verified the shell path reused leader pid 708615 without printing a new autostart message while still routing the command into a subtone.
```

### Logs

```text
logs:
DEBUG: [REPL][ROOM][repl.cmd] {"from":"repl-src-v3-test","message":"probe","room":"index","type":"probe"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Native tailscale already connected via localapi; skipping embedded tsnet startup (tailnet=shad-artichoke.ts.net)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:00.826182397Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"server","room":"index","message":"Leader online on DIALTONE-SERVER (subject=repl.room.index nats=nats://0.0.0.0:42165)","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:00.826210226Z"}
DEBUG: [REPL][ROOM][repl.leader.health] health
DEBUG: [REPL][ROOM][repl.leader.health] health
DEBUG: [REPL][ROOM][repl.cmd] {"type":"command","from":"legion","room":"index","version":"src_v3","os":"linux","arch":"amd64","message":"'proc' 'src_v1' 'emit' 'shell-reuse-ok'","timestamp":"2026-03-18T23:06:02.314946396Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"input","from":"legion","room":"index","message":"/'proc' 'src_v1' 'emit' 'shell-reuse-ok'","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.315198914Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Request received. Spawning subtone for proc src_v1...","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.315218887Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Subtone started as pid 708732.","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.31713953Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Subtone room: subtone-708732","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.317144326Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Subtone log file: /tmp/dialtone-repl-v3-bootstrap-2570621185/repo/.dialtone/logs/subtone-708732-20260318-160602.log","subtone_pid":708732,"log_path":"/tmp/dialtone-repl-v3-bootstrap-2570621185/repo/.dialtone/logs/subtone-708732-20260318-160602.log","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.317145758Z"}
DEBUG: [REPL][ROOM][repl.subtone.708732] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-708732","message":"Started at 2026-03-18T16:06:02-07:00","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.317147475Z"}
DEBUG: [REPL][ROOM][repl.subtone.708732] {"type":"line","scope":"subtone","kind":"lifecycle","room":"subtone-708732","message":"Command: [proc src_v1 emit shell-reuse-ok]","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.317150157Z"}
DEBUG: [REPL][ROOM][repl.subtone.708732] {"type":"line","scope":"subtone","kind":"log","room":"subtone-708732","message":"shell-reuse-ok","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.720134392Z"}
DEBUG: [REPL][ROOM][repl.room.index] {"type":"line","scope":"index","kind":"lifecycle","room":"index","message":"Subtone for proc src_v1 exited with code 0.","subtone_pid":708732,"server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-18T23:06:02.725391416Z"}
PASS: [TEST][PASS] [STEP:shell-routed-command-reuses-running-leader] shell routed command reused existing leader pid 708615
INFO: report: Started the REPL leader first, then ran `./dialtone.sh proc src_v1 emit shell-reuse-ok` and verified the shell path reused leader pid 708615 without printing a new autostart message while still routing the command into a subtone.
PASS: [TEST][PASS] [STEP:shell-routed-command-reuses-running-leader] report: Started the REPL leader first, then ran `./dialtone.sh proc src_v1 emit shell-reuse-ok` and verified the shell path reused leader pid 708615 without printing a new autostart message while still routing the command into a subtone.
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

