# LLM Task: Fix Chrome Process Termination on Windows (dialtone chrome src_v3)

I have a Go-based daemon (`dialtone_chrome_v3.exe`) that launches Google Chrome on a remote Windows host (`legion`) with remote debugging enabled.

## The Problem
The daemon successfully launches Chrome, refines the PID, and connects via `chromedp`. However, within seconds—usually right after the first `Navigate` command—the Chrome process is terminated by the OS. The daemon logs "browser process or port lost" and marks itself unhealthy.

## Current Launch Arguments
The daemon uses these flags:
- `--remote-debugging-port=19464`
- `--remote-debugging-address=127.0.0.1`
- `--user-data-dir=C:\Users\user\.dialtone\chrome-v3\dev`
- `--no-first-run`
- `--no-default-browser-check`
- `--disable-gpu`
- `--headless=new` (tried both with and without)
- `--no-sandbox` (tried both with and without)

## Requirements
1. Investigate if Windows "Job Objects" or "Process Tree Termination" (common in SSH sessions or service managers) is killing child processes when the launcher exits.
2. Determine if `SingletonLock` or `SingletonCookie` in the Chrome Profile needs more aggressive handling.
3. Propose a Go implementation or a PowerShell wrapper that can "detach" the Chrome process from the daemon's process group so the OS doesn't reap it.
4. Verify if specific Windows security policies (like Defender Exploit Protection) might trigger on `--remote-debugging-port` when combined with `--no-sandbox`.

## Relevant Files
- `src/plugins/chrome/src_v3/main.go` (specifically the `ensureBrowser` and `browserArgs` functions).

---

# Chrome src_v3

`chrome src_v3` is the new clean-slate Chrome control path.

Target architecture:

- one daemon process
- one fixed Chrome profile
- one browser-level NATS request/reply connection
- one explicit managed tab id
- no old compatibility behavior
- no implicit relaunch loops

## Current Status

The service logic is complete and supports:

- `./dialtone.sh chrome src_v3 build`
- `./dialtone.sh chrome src_v3 deploy --host <host> --service --role dev`
- `./dialtone.sh chrome src_v3 status --host <host>`
- `./dialtone.sh chrome src_v3 doctor --host <host>`
- `./dialtone.sh chrome src_v3 logs --host <host>`
- `./dialtone.sh chrome src_v3 reset --host <host>`
- `./dialtone.sh chrome src_v3 open --host <host> --url <url>`
- `./dialtone.sh chrome src_v3 goto --host <host> --url <url>`
- `./dialtone.sh chrome src_v3 get-url --host <host>`
- `./dialtone.sh chrome src_v3 tabs --host <host>`
- `./dialtone.sh chrome src_v3 tab-open --host <host> [--url <url>]`
- `./dialtone.sh chrome src_v3 tab-close --host <host>`
- `./dialtone.sh chrome src_v3 close --host <host>`

## Verification on `legion` (Windows)

- **Daemon:** Verified. Starts reliably, manages NATS port 19465.
- **Health Monitoring:** Verified. Correctly detects and reports browser death.
- **PID Tracking:** Verified. Correctly refines the real browser PID after launch.
- **Action Commands:** `open` succeeds initially, but the browser is frequently killed by the OS shortly after.
- **Reliability:** The service correctly marks itself unhealthy when the browser is lost and requires a `reset` or manual intervention, avoiding hidden relaunch loops.

## Known Windows Issue

On the `legion` host, Chrome instances launched by the service are frequently terminated by the OS shortly after establishing a debug connection. This is likely due to security policies or process job management on that specific host. The `src_v3` logic correctly identifies these failures and reports them via `status` and `doctor`.
