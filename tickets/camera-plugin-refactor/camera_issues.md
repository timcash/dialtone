# Camera Streaming Issues & Debugging Summary

## Current Status
**System is non-operational.** The camera stream fails to initialize or maintain stability, frequently citing "Device or resource busy" errors despite extensive software mitigation attempts.

## Symptoms
1.  **"Device or resource busy"**: The most persistent error. Occurs during:
    *   Initial startup (Auto-start).
    *   Manual restarts via the dashboard button.
    *   Redeployment of the `dialtone` service.
2.  **Multi-Viewer Crashes**: Direct streaming mode crashes immediately when a second client connects.
3.  **Zombie Processes**: Older `dialtone` processes were found holding the `/dev/video0` handle even after `pkill`.

## Hardware Context
-   **Device**: L01 HD Webcam (`/dev/video0`).
-   **Driver**: `uvcvideo` (standard Linux USB Video Class).
-   **Capabilities**: Supports MJPEG and YUYV. Direct MJPEG pass-through is desired for performance.
-   **Constraint**: The hardware does NOT support concurrent access (e.g., multiple file handles).

## Attempted Solutions & Outcomes

### 1. Direct Streaming (Pre-Fanout)
-   **Approach**: `StreamHandler` reads directly from `cam.GetFrames()`.
-   **Outcome**: Works for **one** viewer. Crashes on **two** viewers because the hardware cannot be shared.
-   **Verdict**: Insufficient for requirements.

### 2. Buffered Streaming ("Fanout")
-   **Approach**: Single background goroutine captures frames to a global `latestFrame` buffer. Clients poll this buffer.
-   **Outcome**: Solves multi-viewer concurrency. However, introduced race conditions where the capture loop would fight with restart logic for the device handle.

### 3. Safe Restart & Sync
-   **Approach**: Added `sync.WaitGroup` to ensure the capture loop fully exits before `RestartCamera` attempts to close/re-open.
-   **Outcome**: Reduced race conditions but did not eliminate "Device busy" on rapid restarts.

### 4. Leak Prevention
-   **Approach**: Split `device.Open` and `SetPixFormat`. If format setup fails (common source of EBUSY), explicitly Close the handle to prevent FD leaks.
-   **Outcome**: Prevents "self-inflicted" DOS, but does not solve the underlying hardware lock if the kernel driver is stuck.

### 5. Deployment Hardening
-   **Approach**: Modified `deploy.go` to loop and wait for `pgrep dialtone` to return empty before starting the new service.
-   **Outcome**: Confirmed old process is dead, yet "Device busy" sometimes persists immediately after start.

## Root Cause Analysis
-   **Primary Suspect**: The `go4vl` library or the underlying `v4l2` interaction is leaving the device in a "streaming" state even after the file descriptor is closed.
-   **Secondary Suspect**: The USB hardware itself is getting into a bad state that requires a power cycle (USB reset), which software restarts cannot fix.
-   **Deployment Race**: Even with checks, the kernel might take milliseconds to release the device after the process dies, causing the new process (which starts immediately) to find it busy.

## Next Steps
1.  **Hardware Reset**: Implement a USB reset (via `ioctl` or `usbreset` utility) before `device.Open`.
2.  **Kernel Logs**: Check `dmesg` on the robot for `uvcvideo` driver errors.
3.  **Alternative Library**: If `go4vl` proves unstable, shell out to `ffmpeg` or `mjpeg-streamer` for the raw capture and proxy the stream.
