## Chrome Fix

Observed problem:
- the Chrome daemon should own one Chrome instance per role and keep it alive
- instead, CAD/UI flows could trigger extra launch behavior and create confusion between an old browser window and a new one
- repeated deploy/start behavior also caused repeated Windows trust prompts for `dialtone_chrome_v3.exe`

Target contract:
- `chrome src_v3 service --mode start` may launch Chrome only when the role has no healthy daemon/browser
- `chrome src_v3 status` must never launch Chrome
- `chrome src_v3 reset` must reset the managed tab/session, not spawn a new browser
- `chrome src_v3 open` must reuse the browser already owned by the daemon
- CAD/UI/browser tests should reuse a healthy daemon first and only redeploy on explicit maintenance flows

Immediate fixes:
- reuse existing remote Chrome daemons in CAD/UI test preflight
- only fall back to deploy/start when `status` fails
- honor command-specific timeout values for longer browser waits over NATS request/reply

Next daemon fixes:
- persist and trust one browser PID per role
- refuse to start a second browser while the tracked browser PID is still alive
- make tab reset the default behavior for tests
- keep `restart` as the explicit browser-relaunch path

Verification:
- start `cad-smoke` once and reuse it for CAD browser smoke runs
- start `dev` once and reuse it for direct Chrome actions
- confirm `status`, `open`, `reset`, `click`, `type`, and `screenshot` operate against the same daemon-owned browser

Debug workflow for an LLM:

```bash
# 1. Reset the local REPL/process-manager state first.
./dialtone.sh repl src_v3 process-clean

# 2. Start one long-lived Chrome daemon role on legion.
./dialtone.sh chrome src_v3 service --host legion --mode start --role cad-smoke

# 3. Confirm the daemon is healthy over the normal REPL/NATS path.
./dialtone.sh chrome src_v3 status --host legion --role cad-smoke

# 4. Exercise the existing daemon without redeploying it.
./dialtone.sh chrome src_v3 screenshot --host legion --role cad-smoke --out src/plugins/chrome/src_v3/screenshots/cad_smoke_check.png

# 5. Run the focused CAD smoke against that already-running role.
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke

# 6. If the smoke fails, inspect the subtone log instead of guessing from DIALTONE>.
./dialtone.sh repl src_v3 subtone-list --count 10
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 250
```

What to look for:
- `chrome status: daemon ready on legion role=cad-smoke`
- no `chrome src_v3 build ok` / redeploy lines during normal CAD/UI preflight
- one persistent daemon/browser per role
- CAD failures narrowed to UI/model-ready state, not Chrome lifecycle
