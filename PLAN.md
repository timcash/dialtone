# Chrome `src_v3` Plan

This file captures the current state of the `chrome src_v3` work, the exact Windows-to-WSL workflow that has been used so far, and the next steps to make the normal service-managed path reliable.

## Goal

Make `./dialtone.sh chrome src_v3 ...` reliable when driven from WSL through the REPL, while the actual Chrome daemon runs on the Windows host.

Desired model:

- one long-lived daemon per role
- one managed tab per role
- one command at a time over NATS
- CLI commands sent through the REPL
- predictable logs and screenshots
- `./dialtone.sh chrome src_v3 test` should pass without any manual daemon bootstrapping

## What Is Working

These parts are now proven:

- REPL-routed `chrome src_v3 status` works against a live Windows daemon.
- REPL-routed `chrome src_v3 open` works and reuses the managed tab.
- The daemon can keep the browser alive and process commands sequentially.
- Screenshot capture itself works on Windows.
- `chrome src_v3 test-actions` can now pass end-to-end when a live daemon is already running.
- The screenshot path no longer depends on large inline NATS payloads.

Important design changes already made:

- `src/plugins/chrome/src_v3/daemon.go`
  - added explicit request serialization with `reqMu`
  - screenshot now writes a managed artifact path and can avoid large inline payloads
- `src/plugins/chrome/src_v3/cli.go`
  - screenshot output can be fetched from the remote host after the NATS reply
  - `test-actions` uses the same artifact-aware path
- `src/plugins/chrome/src_v3/test/cmd/main.go`
  - screenshot test flow now accepts either inline screenshot data or artifact-path responses
- `src/plugins/ssh/src_v1/go/mesh.go`
  - PowerShell transport now sets `$ProgressPreference='SilentlyContinue'` to reduce CLIXML progress noise

## Main Finding

The browser logic is no longer the main problem.

The current remaining weak spot is the Windows service start path used by:

- `EnsureRemoteServiceByHost`
- `startRemoteService`
- `waitForRemoteService`

In practice:

- a manually started Windows daemon responds well
- the standard service-managed launcher still behaves inconsistently
- the daemon process can appear to start, but the WSL side does not always treat it as ready soon enough

## Recommended Workflow

Use the Windows repo for editing and the WSL repo for runtime/testing.

Working roots:

- Windows repo: `C:\Users\timca\dialtone`
- WSL repo: `/home/user/dialtone`
- tmux session: `windows`

## `wsl-tmux` Usage

The preferred way to drive WSL from Windows is:

```powershell
wsl-tmux help
wsl-tmux status
wsl-tmux clean-state
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 status --host legion --role dev"
wsl-tmux read
wsl-tmux interrupt
```

Important behaviors:

- `wsl-tmux` with no args reads the pane
- unknown first arg is treated as a command to send
- `clean-state` clears the pane and leaves the tmux session alive
- `interrupt` sends `Ctrl-C` without killing the tmux session

Use `wsl-tmux status` before sending more commands if the pane looks stale.

## Important Commands

### Basic WSL / REPL checks

```powershell
wsl-tmux clean-state
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 process-clean"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 status --host legion --role dev"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 open --host legion --role dev --url https://example.com"
```

### Action / screenshot smoke check

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 test-actions --host legion --role dev --out /tmp/chrome-actions.png"
wsl-tmux "ls -l /tmp/chrome-actions.png && file /tmp/chrome-actions.png"
```

If this succeeds, the daemon is good enough for real UI command flow.

### Full suite

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 test --host legion --role dev"
```

The suite report lives at:

- `/home/user/dialtone/src/plugins/chrome/src_v3/TEST.md`
- `/home/user/dialtone/src/plugins/chrome/src_v3/TEST_RAW.md`

### Read latest subtone output

The pane already shows the subtone log path. To read it again:

```powershell
wsl-tmux read
```

Or directly in WSL once you know the path:

```powershell
wsl-tmux "tail -n 120 /home/user/.dialtone/logs/<subtone-log>.log"
```

## Manual Debug Workflow

This is useful only when the normal service-managed launcher is under investigation.

### 1. Stop old Windows chrome daemons

```powershell
Get-Process dialtone_chrome_v3 -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process chrome -ErrorAction SilentlyContinue | Where-Object { $_.Path -like 'C:\Program Files\Google\Chrome\Application\chrome.exe' } | Stop-Process -Force
```

### 2. Deploy the latest Windows binary from WSL

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 deploy --host legion --role dev"
```

### 3. Start a manual Windows daemon

```powershell
$bin = 'C:\Users\timca\.dialtone\bin\dialtone_chrome_v3.exe'
$out = 'C:\Users\timca\.dialtone\bin\chrome-dev-manual.out.log'
$err = 'C:\Users\timca\.dialtone\bin\chrome-dev-manual.err.log'

Start-Process -FilePath $bin `
  -ArgumentList @(
    'src_v3', 'daemon',
    '--role', 'dev',
    '--chrome-port', '19464',
    '--host-id', 'legion',
    '--nats-url', 'nats://dialtone-server-wsl.shad-artichoke.ts.net:4222'
  ) `
  -WorkingDirectory 'C:\Users\timca\.dialtone\bin' `
  -RedirectStandardOutput $out `
  -RedirectStandardError $err `
  -WindowStyle Hidden
```

### 4. Validate it from WSL

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 status --host legion --role dev"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 test-actions --host legion --role dev --out /tmp/chrome-actions.png"
```

This path has already been useful for proving that:

- the daemon can answer NATS requests
- the managed tab model is sound
- screenshot capture is sound
- the remaining problems are in service startup / readiness, not the browser command loop

## Current Known Good / Known Bad

### Known good

- manual Windows daemon + REPL-routed `status`
- manual Windows daemon + REPL-routed `open`
- manual Windows daemon + REPL-routed `test-actions`
- screenshot artifact fetch back into WSL

### Still needs work

- the standard Windows launcher path used by `startRemoteService`
- the readiness detection in `waitForRemoteService`
- the standard daemon log path used by `readRemoteLogs`
- full `chrome src_v3 test` with no manual intervention

## Best Next Steps

1. Finish the standard Windows launcher.

What to check:

- does `sshv1.RunNodeCommand(... Start-Process ...)` return promptly every time?
- if the daemon starts, why does `waitForRemoteService` still miss readiness?
- should readiness accept `state.json` plus process existence before demanding a successful NATS `status` round-trip?

2. Move Windows daemon logs to a home-scoped service directory.

Recommended target:

- `C:\Users\timca\.dialtone\chrome-v3\<role>\service\daemon.out.log`
- `C:\Users\timca\.dialtone\chrome-v3\<role>\service\daemon.err.log`

Reason:

- it matches the Linux/local layout
- it avoids reusing ad hoc log files in the binary directory
- it should make `readRemoteLogs` and test expectations clearer

3. Re-run the full service-managed suite without any manual daemon process.

Target command:

```powershell
wsl-tmux clean-state
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh chrome src_v3 test --host legion --role dev"
```

4. Once the standard path passes:

- update `src/plugins/chrome/src_v3/README.md`
- update `src/plugins/chrome/src_v3/TEST.md`
- commit and push

## Current Investigation Notes

- The daemon process can start on Windows and write `state.json` under:
  - `C:\Users\timca\.dialtone\chrome-src-v3\dev\state.json`
- This suggests the daemon itself is alive even when the suite is still waiting.
- That means the most likely remaining issue is the launcher/readiness contract, not the core browser command loop.

## Short Summary

The chrome command loop is much closer to the desired architecture now.

What is basically solved:

- one daemon
- one managed tab
- REPL-routed commands
- screenshot capture
- remote artifact fetch for screenshots

What still needs to be hardened:

- reliable Windows service launch from the normal `chrome src_v3` path
- reliable readiness detection
- clean daemon log handling on Windows

Until that is done, the manual daemon flow is the best way to prove the browser-side behavior while the launcher is being repaired.
