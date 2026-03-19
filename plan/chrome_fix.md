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
