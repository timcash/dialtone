# Chrome src_v3

`chrome src_v3` is the current Dialtone remote Chrome control path.

It is designed around:
- one daemon per role
- one long-lived Chrome process
- one preserved Chrome profile per role
- one managed tab reused by default
- one NATS request/reply control surface

## Runtime Note

Plain `./dialtone.sh chrome src_v3 ...` is the default operator path.

That command is normally routed through the local REPL leader, which means:
- `DIALTONE>` should stay high-level
- full request detail stays in the subtone log
- use `./dialtone.sh repl src_v3 subtone-list --count 20`
- use `./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200`

Typical index-room summaries are:

```text
DIALTONE> chrome service: ensuring daemon on legion role=dev
DIALTONE> chrome goto: opening http://127.0.0.1:8766/chrome_src_v3_action.html on legion role=dev
DIALTONE> chrome screenshot: saved to /home/user/dialtone/src/plugins/chrome/src_v3/screenshots/manual_debug.png
```

The normal shell transcript pattern is:

```text
legion> /chrome src_v3 screenshot --host legion --role dev --out src/plugins/chrome/src_v3/screenshots/manual_debug.png
DIALTONE> Request received. Spawning subtone for chrome src_v3...
DIALTONE> Subtone started as pid 327530.
DIALTONE> Subtone room: subtone-327530
DIALTONE> Subtone log file: /home/user/dialtone/.dialtone/logs/subtone-327530-20260317-171019.log
DIALTONE> chrome screenshot: capturing managed tab on legion role=dev
DIALTONE> chrome service: ensuring daemon on legion role=dev
DIALTONE> chrome screenshot: saved to /home/user/dialtone/src/plugins/chrome/src_v3/screenshots/manual_debug.png
DIALTONE> Subtone for chrome src_v3 exited with code 0.
```

`--host` is plugin-local for `chrome` and should keep meaning the target mesh node, for example `legion`.

## Shell Workflow

```sh
# From repo root
cd /home/user/dialtone

# Deploy and start a headed role on legion
./dialtone.sh chrome src_v3 deploy --host legion --role dev --service

# Check service state
./dialtone.sh chrome src_v3 status --host legion --role dev
./dialtone.sh chrome src_v3 logs --host legion
./dialtone.sh chrome src_v3 doctor --host legion

# Drive the managed tab
./dialtone.sh chrome src_v3 goto --host legion --role dev --url http://127.0.0.1:8766/chrome_src_v3_action.html
./dialtone.sh chrome src_v3 type-aria --host legion --role dev --label "Name Input" --value dialtone
./dialtone.sh chrome src_v3 click-aria --host legion --role dev --label "Do Thing"

# Inspect live DOM state on the managed tab
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role dev --label "Name Input" --attr value
./dialtone.sh chrome src_v3 wait-log --host legion --role dev --contains "clicked:" --timeout-ms 5000

# Capture a screenshot
./dialtone.sh chrome src_v3 screenshot --host legion --role dev --out src/plugins/chrome/src_v3/screenshots/manual_debug.png
```

## Core Model

Normal behavior:
- keep one Chrome process running on the target host
- keep one managed tab
- reuse that tab across tests and dev flows
- preserve the Chrome user-data directory

Recovery behavior:
- if the managed target is gone, stale, or unhealthy, recreate the managed tab
- if the browser process is gone, restart the browser service

What should not happen by default:
- creating a new tab for every test
- deleting the Chrome user-data directory during normal reset/deploy
- silently running multiple independent headed sessions against one role on the same host

## Main Commands

Lifecycle:
- `./dialtone.sh chrome src_v3 deploy --host <host> --role <role> --service`
- `./dialtone.sh chrome src_v3 service --host <host> --mode start|stop|status --role <role>`
- `./dialtone.sh chrome src_v3 status --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 doctor --host <host>`
- `./dialtone.sh chrome src_v3 logs --host <host>`
- `./dialtone.sh chrome src_v3 reset --host <host>`

Navigation:
- `./dialtone.sh chrome src_v3 open --host <host> --role <role> --url <url>`
- `./dialtone.sh chrome src_v3 goto --host <host> --role <role> --url <url>`
- `./dialtone.sh chrome src_v3 get-url --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 tabs --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 tab-open --host <host> --role <role> [--url <url>]`
- `./dialtone.sh chrome src_v3 tab-close --host <host> --role <role> [--index <n>]`
- `./dialtone.sh chrome src_v3 close --host <host> --role <role>`

Element actions:
- `./dialtone.sh chrome src_v3 click-aria --host <host> --role <role> --label <aria-label>`
- `./dialtone.sh chrome src_v3 type-aria --host <host> --role <role> --label <aria-label> --value <text>`
- `./dialtone.sh chrome src_v3 wait-aria --host <host> --role <role> --label <aria-label> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 wait-aria-attr --host <host> --role <role> --label <aria-label> --attr <name> --expected <value> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 get-aria-attr --host <host> --role <role> --label <aria-label> --attr <name>`

Debugging:
- `./dialtone.sh chrome src_v3 console --host <host> --role <role>`
- `./dialtone.sh chrome src_v3 wait-log --host <host> --role <role> --contains <text> [--timeout-ms 5000]`
- `./dialtone.sh chrome src_v3 screenshot --host <host> --role <role> --out <png-path>`

## Roles

Use roles to isolate long-lived browser sessions:
- `robot-test`: integrated robot suite on `legion`
- `robot-dev`: live dev browser for `robot src_v2 dev`
- `dev`: generic/manual use (default)

Recommendation:
- keep one role per workflow
- do not mix unrelated tests on the same role
- do not run concurrent headed flows against the same role unless you explicitly want them to share one tab

## Agent & System Internals

This section provides critical context for LLM agents and automated tools.

### Process & Port Mapping
- **Daemon Port (NATS)**: `19465` (default). This is the control plane.
- **Chrome Debug Port**: `19464` (default). The daemon communicates with Chrome over this port via `chromedp`.
- **Isolation**: Each role gets its own NATS port and Chrome port if multiple daemons run on one host (though standard practice is one daemon/role per host).

### Log Streams
1. **Daemon Logs**: Stored locally on the host at `~/.dialtone/chrome-v3/<role>/service/daemon.out.log`. These logs show NATS request handling and browser lifecycle events.
2. **Browser Console**: The daemon captures the last 200 lines of the browser's console. Query these via `./dialtone.sh chrome src_v3 console`.
3. **REPL Subtone Logs**: Plain shell commands keep the detailed CLI-side request/response output in the subtone log shown in the `DIALTONE>` transcript.
4. **NATS Transport**: All CLI commands are converted to NATS requests on the subject `chrome.src_v3.<role>.cmd`. Responses include status, current URL, and console log snapshots.

### LLM Strategy for Troubleshooting
- **Verification**: Always start with `status`. If `unhealthy=true`, check the daemon logs.
- **State Recovery**: If the browser is unresponsive, try `reset`. This kills the browser and clears lock files but preserves the profile.
- **DOM Inspection**: Use `get-aria-attr` and `screenshot` to verify UI state without needing a head.
- **Synchronicity**: Use `wait-log` or `wait-aria` to handle async page loads. The system is designed for high-latency remote links.
- **REPL Context Hygiene**: Keep the index room clean. Use `DIALTONE>` lifecycle plus short summaries for progress, and use `subtone-log` when raw payloads or stack traces are needed.

## Integration With `test/src_v1`

`chrome src_v3` is the preferred remote headed-browser backend for Dialtone tests.

What the test library expects:
- one attach node, often `legion` on WSL
- one attach role, for example `robot-test`
- one reusable managed tab

Current robot pattern:
- local UI/mock server on WSL
- remote Chrome on `legion`
- rover backend optionally on a third host

Example:
```sh
./dialtone.sh robot src_v2 test --filter three-system-arm
./dialtone.sh robot src_v2 test --filter local-ui-mock-e2e
```

## Lifecycle Rules

- `open` and `goto` reuse the managed tab.
- `deploy --service` preserves the running browser if the remote binary is already current.
- `reset` preserves the Chrome profile/user-data directory.
- explicit `tab-open`, `tab-close`, or `close` are the normal commands that change tab/browser lifecycle on purpose.

## Verification On `legion`

Current working behavior on `legion`:
- headed browser stays up across robot test runs
- managed tab can be reused across dev and test flows by role
- DOM attrs can be read remotely with `get-aria-attr`
- screenshots and console logs can be collected remotely

Known operational constraint:
- a role is effectively single-session. If two workflows drive the same role at once, they are sharing one managed tab and will interfere with each other.

## Troubleshooting

1. Check daemon state:
```sh
./dialtone.sh chrome src_v3 status --host legion --role dev
```

2. If the tab lands on `chrome-error://chromewebdata/`, verify the target UI server is actually running.

3. Read browser console:
```sh
./dialtone.sh chrome src_v3 console --host legion --role dev
```

4. Inspect live UI attrs:
```sh
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role dev --label "Name Input" --attr value
```

5. Only use `reset` when normal recovery is not enough:
```sh
./dialtone.sh chrome src_v3 reset --host legion
```

6. Inspect the exact subtone transcript when the index room is not enough:
```sh
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

## Related Docs

- [README.md](/home/user/dialtone/src/plugins/test/src_v1/README.md)
- [README.md](/home/user/dialtone/src/plugins/robot/src_v2/README.md)
