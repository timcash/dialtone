# WSL Terminal + Chrome E2E Plan

This plan replaces the old broad repo plan with one focused end-to-end test plan for the Windows host, the WSL terminal bootstrap path, and the Chrome `src_v3` daemon warmup flow.

## Goal

From Windows, one command should make the developer environment feel ready:

- the target WSL distro is running
- a real Windows desktop terminal opens
- that terminal lands inside the WSL Dialtone repo
- the shell prints a short usage banner
- the Chrome `src_v3` service for `host=legion role=dev` is being warmed automatically through the README-supported deploy path
- the user can enter the WSL `dialtone>` REPL and immediately drive Chrome tests through the normal plugin workflow

## Default Assumptions

- Windows repo: `C:\Users\timca\dialtone`
- WSL repo: `/home/user/dialtone`
- WSL distro: `Ubuntu-24.04`
- Chrome service host: `legion`
- Chrome service role: `dev`

If those change, update the matching config or command flags before rerunning this plan.

## Main Entry Commands

Windows host lifecycle and terminal bootstrap:

```powershell
.\dialtone.ps1 wsl src_v3 status
.\dialtone.ps1 wsl src_v3 stop --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 start --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 terminal --name Ubuntu-24.04
```

WSL-side Chrome and REPL checks:

```bash
cd /home/user/dialtone
./dialtone.sh chrome src_v3 status --host legion --role dev
./dialtone.sh chrome src_v3 deploy --host legion --role dev --service
./dialtone.sh
```

## End-To-End Scenarios

### 1. Windows Host Control Works Locally

Run:

```powershell
.\dialtone.ps1 wsl src_v3 status
.\dialtone.ps1 wsl src_v3 stop --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 status
.\dialtone.ps1 wsl src_v3 start --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 status
```

Expect:

- `status` talks directly to `wsl.exe` from Windows
- `stop` moves the distro to `Stopped`
- `start` moves the distro back to `Running`
- the command path does not depend on first entering the target distro shell

### 2. Terminal Bootstrap Opens A Real Desktop Shell

Run:

```powershell
.\dialtone.ps1 wsl src_v3 terminal --name Ubuntu-24.04
```

Expect in the new desktop terminal window:

- the window opens even if the distro had been stopped
- the shell starts inside `/home/user/dialtone`
- the banner explains:
  `Run ./dialtone.sh to enter the dialtone> repl.`
- the shell is interactive and ready for normal Linux commands immediately

### 3. Chrome Warmup Is Triggered Automatically

From the terminal window opened above, or from another WSL shell, run:

```bash
cd /home/user/dialtone
./dialtone.sh chrome src_v3 status --host legion --role dev
```

Also inspect the warmup log when needed:

```bash
tail -n 120 ~/.dialtone/logs/wsl-terminal-chrome-legion-dev.log
```

Expect:

- the terminal banner says warmup was queued
- the warmup path uses the README-supported service command:
  `./dialtone.sh chrome src_v3 deploy --host legion --role dev --service`
- after warmup settles, `status` reports a healthy daemon/browser role on `legion`

### 4. The WSL REPL Can Reuse The Warmed Chrome Role

Inside the WSL terminal:

```bash
cd /home/user/dialtone
./dialtone.sh
```

Then in the REPL:

```text
/chrome src_v3 status --host legion --role dev
/chrome src_v3 goto --host legion --role dev --url about:blank
/chrome src_v3 get-url --host legion --role dev
```

Expect:

- `dialtone>` comes up normally
- the Chrome role is already running or is reused without manual daemon bootstrapping
- the managed tab commands succeed through the normal REPL + service path

### 5. Visible WSL Test Sweep

Run from Windows with the visible tmux helper:

```powershell
.\dialtone.ps1 tmux clean-state
.\dialtone.ps1 tmux "./dialtone.sh chrome src_v3 status --host legion --role dev"
.\dialtone.ps1 tmux "./dialtone.sh chrome src_v3 test-actions --host legion --role dev"
.\dialtone.ps1 tmux read
```

Expect:

- the visible tmux session shows the same `legion/dev` service state the terminal bootstrap prepared
- `test-actions` can reuse the warmed browser service instead of creating a disconnected ad hoc session

## Failure Checks

If a scenario fails, capture these first:

```powershell
.\dialtone.ps1 wsl src_v3 status
```

```bash
cd /home/user/dialtone
./dialtone.sh chrome src_v3 status --host legion --role dev
./dialtone.sh chrome src_v3 logs --host legion
./dialtone.sh chrome src_v3 doctor --host legion
tail -n 120 ~/.dialtone/logs/wsl-terminal-chrome-legion-dev.log
```

## Acceptance Criteria

This feature set is done when all of these are true in the same branch:

- `.\dialtone.ps1 wsl src_v3 terminal --name Ubuntu-24.04` starts the distro if needed
- that command opens a real desktop terminal into the WSL repo root
- the banner tells the user how to enter the REPL
- Chrome warmup for `legion/dev` is queued automatically from the terminal bootstrap
- the WSL REPL can immediately drive `chrome src_v3` against that warmed role
- the focused Go tests for the WSL plugin keep passing

## Focused Verification Commands

Go package verification:

```powershell
cd C:\Users\timca\dialtone\src
& 'C:\Program Files\Go\bin\go.exe' test ./plugins/wsl/src_v3/go ./plugins/wsl/scaffold
```

Manual host verification:

```powershell
cd C:\Users\timca\dialtone
.\dialtone.ps1 wsl src_v3 stop --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 terminal --name Ubuntu-24.04
.\dialtone.ps1 wsl src_v3 status
```
