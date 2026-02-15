# Cloudflare Tunnel Status and Troubleshooting

This document outlines the current state of Cloudflare tunnel integration within Dialtone, based on recent interactions.

## Summary of Functionality:

### What Works:
*   **Robot Web UI:** The robot's web UI is confirmed to be running on `http://drone-1` (port 80) and is accessible directly via Tailscale from your local machine.
*   **Cloudflare Tunnel Provisioning:** The `./dialtone.sh cloudflare provision <name>` command successfully:
    *   Creates a named Cloudflare Tunnel in your Cloudflare account.
    *   Creates the corresponding DNS `CNAME` record (e.g., `drone-1.dialtone.earth` pointing to `[tunnel-id].cfargotunnel.com`).
    *   Generates and saves the `CF_TUNNEL_TOKEN_<NAME>` to your `env/.env` file.
*   **Foreground Execution of `robot` command:** When `./dialtone.sh cloudflare robot drone-1` is run directly (in the foreground), the Cloudflare tunnel *successfully establishes* and the robot's web page becomes accessible at `https://drone-1.dialtone.earth`.

### What Doesn't Work Consistently (or is Problematic):
*   **Background Persistence:** When `./dialtone.sh cloudflare robot drone-1` is run in the background (using `&`), it leads to Cloudflare "Error 1033 Ray ID:..." (Cloudflare Tunnel error) after some time, or immediately. This indicates the `cloudflared` process, or its parent `dialtone` process, is not remaining active as expected in the background.

## Current Understanding of the "Backgrounding" Issue:

The `dialtone.sh` script is designed to run long-lived Go programs in the background (using `dialtone.sh proc ps` for tracking). The `runRobot` Go command, when executed, starts the `cloudflared` binary.

*   **Expected Behavior:**
    1.  `./dialtone.sh cloudflare robot drone-1 &` starts `dialtone.sh` in the background.
    2.  `dialtone.sh` starts the Go `runRobot` command.
    3.  `runRobot` starts `cloudflared` using `cmd.Start()`.
    4.  `runRobot` then calls `util.WaitForShutdown()`, which should keep the Go process (and thus `dialtone.sh`) alive and tracking `cloudflared` until a signal is received.
    5.  `cloudflared` (running as a child of the long-lived Go process) should maintain its connection to Cloudflare.

*   **Observed Behavior:** The `cloudflared` process (or its `dialtone` parent) is exiting prematurely when backgrounded, causing the "Error 1033" because the Cloudflare edge can no longer reach the local `cloudflared` instance.

## Proposed Next Steps to Address Backgrounding:

The most robust way to ensure a daemonized process like `cloudflared` persists is to properly detach it from its parent processes.

The `runRobot` function needs to ensure that `cloudflared` is executed in such a way that it becomes a completely independent background daemon, and the `dialtone` Go process can then exit gracefully (or manage the daemonized process).

However, `cloudflared` itself does not have a native "daemonize" flag for `tunnel run`. So, the parent Go process must manage it. The `dialtone.sh` `run_tool` is expecting the Go process to be long-running.

The immediate fix is to ensure the `cloudflared` process started by `runRobot` is properly detached and handled.

**1. Ensuring `cloudflared` is correctly installed:**
The `cloudflared: command not found` error was a past issue. We need to ensure `cloudflared` is correctly installed before proceeding.
*(To be verified once the `install` command can be rerun successfully after fixing pending Go compilation errors).*

**2. Verifying Process Detachment:**
The `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}` in `runRobot` is intended to detach `cloudflared` into its own process session. This is the correct approach for daemonization from a Go program.

The subsequent `util.WaitForShutdown()` means the `dialtone` Go program stays alive, acting as a supervisor that keeps the `cloudflared` process running and allows `dialtone.sh` to track this supervising process.

If "Error 1033" still occurs, it strongly suggests:
*   `cloudflared` is crashing immediately after `cmd.Start()` and before `util.WaitForShutdown()` gets a chance to stabilize.
*   The `CF_TUNNEL_TOKEN_DRONE_1` might be getting corrupted or not passed correctly when backgrounded (less likely if it works in foreground).
*   There's an underlying network issue or Cloudflare configuration problem that only manifests when `cloudflared` tries to persist connections over time.

Given the new `runRobot` implementation using `cmd.Start()` and `util.WaitForShutdown()`, the backgrounding logic *should* be correct. The persistence problem might be deeper within `cloudflared`'s interaction with the system or the Cloudflare API when run as a background service.

A thorough test would be to manually run `cloudflared tunnel run --token ... --url ...` outside of `dialtone.sh` and try to background it (`nohup ... &`) to see if it persists.
