# Error Report

- **Date**: Sat, 14 Mar 2026 16:57:00 PDT
- **Suite**: repl-src-v3
- **Total Duration**: 48.021879s

- **Error Steps**: 1 / 6

## 6. interactive-cloudflare-tunnel-start

- **Duration**: 40.593070875s
- **Step Error**: `transcript step 1 room expect failed: timeout waiting for room patterns: Subtone for cloudflare src_v1 exited with code 0.
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
{"type":"heartbeat","room":"index","message":"alive","server_id":"DIALTONE-SERVER@index","timestamp":"2026-03-14T23:56:59.848551Z"}`

### Step Errors

```text
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

---

