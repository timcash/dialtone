# Chrome src_v3

`chrome src_v3` is the current Dialtone remote Chrome control path.

It is designed around:
- one daemon per role
- one long-lived Chrome process
- one preserved Chrome profile per role
- one managed tab reused by default
- one NATS request/reply control surface

## Shell Workflow

```sh
# From repo root
cd /home/user/dialtone

# Build local binaries
./dialtone.sh chrome src_v3 build

# Deploy and start the robot-test role on legion
./dialtone.sh chrome src_v3 deploy --host legion --role robot-test --service

# Check service state
./dialtone.sh chrome src_v3 status --host legion --role robot-test
./dialtone.sh chrome src_v3 logs --host legion
./dialtone.sh chrome src_v3 doctor --host legion

# Drive the managed tab
./dialtone.sh chrome src_v3 goto --host legion --role robot-test --url http://127.0.0.1:3000/#robot-three-stage
./dialtone.sh chrome src_v3 click-aria --host legion --role robot-test --label "Three Mode"
./dialtone.sh chrome src_v3 click-aria --host legion --role robot-test --label "Three Thumb 1"
./dialtone.sh chrome src_v3 wait-log --host legion --role robot-test --contains "Publishing rover.command cmd=arm" --timeout-ms 5000

# Inspect live DOM state on the managed tab
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-status-text
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-command-ack-result

# Capture a screenshot
./dialtone.sh chrome src_v3 screenshot --host legion --role robot-test --out src/plugins/chrome/src_v3/screenshots/manual_debug.png
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
- `./dialtone.sh chrome src_v3 build`
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
- `dev`: generic/manual use

Recommendation:
- keep one role per workflow
- do not mix unrelated tests on the same role
- do not run concurrent headed flows against the same role unless you explicitly want them to share one tab

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
./dialtone.sh chrome src_v3 status --host legion --role robot-test
```

2. If the tab lands on `chrome-error://chromewebdata/`, verify the target UI server is actually running.

3. Read browser console:
```sh
./dialtone.sh chrome src_v3 console --host legion --role robot-test
```

4. Inspect live UI attrs:
```sh
./dialtone.sh chrome src_v3 get-aria-attr --host legion --role robot-test --label "Xterm Terminal" --attr data-last-error-line
```

5. Only use `reset` when normal recovery is not enough:
```sh
./dialtone.sh chrome src_v3 reset --host legion
```

## Related Docs

- [README.md](/home/user/dialtone/src/plugins/test/src_v1/README.md)
- [README.md](/home/user/dialtone/src/plugins/robot/src_v2/README.md)
