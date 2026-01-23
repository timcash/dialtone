# Camera Debugging Workflow

```bash
# 1. Build locally to verify syntax
./dialtone.sh build --local

# 2. Deploy to robot (auto-cross-compile)
./dialtone.sh deploy

# 3. Run health diagnostics (local & remote)
./dialtone.sh diagnostic

# 4. Monitor logs for "System Operational" & "Camera capture loop started"
./dialtone.sh logs --remote

# 4. Test Stream Connection (Response should contain multipart/x-mixed-replace)
curl -v http://100.71.101.126:80/stream

# 5. Test Graceful Restart
curl -v http://100.71.101.126:80/api/camera/restart

# 6. Run Diagnostic Leak Test
# Cross-compile the test tool
podman run --rm -v $(pwd):/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=arm64 -e CGO_ENABLED=1 -e CC=aarch64-linux-gnu-gcc dialtone-builder bash -c "go build -buildvcs=false -o bin/cam_leak_test src/cmd/cam_leak_test/main.go"

# Deploy test tool
scp bin/cam_leak_test tim@192.168.4.36:~/cam_leak_test

# Run test tool (stops service first)
ssh tim@192.168.4.36 "chmod +x ~/cam_leak_test && pkill dialtone || true && ~/cam_leak_test"

# 7. Run Camera Hardware Unit Test (src/camera_linux_test.go)
# Cross-compile the standard Go test suite into a binary
podman run --rm -v $(pwd):/src:Z -v dialtone-go-build-cache:/root/.cache/go-build:Z -w /src -e GOOS=linux -e GOARCH=arm64 -e CGO_ENABLED=1 -e CC=aarch64-linux-gnu-gcc dialtone-builder bash -c "go test -c -tags=linux,cgo -o bin/camera_test ./src"

# Deploy test binary
scp bin/camera_test tim@192.168.4.36:~/camera_test

# Run the test on the robot
ssh tim@192.168.4.36 "chmod +x ~/camera_test && pkill dialtone || true && ~/camera_test -test.v -test.run TestCamera_HardwareCapture"
```

# Camera Debugging Status & Findings

## Current Technical State
-   **Service Status**: The `dialtone` service is running on the robot (`192.168.4.36`).
-   **Logs**: Remote logs (`logs --remote`) show successful startup.
    -   "System Operational"
    -   "Camera capture loop started" (appears when stream is requested)
    -   Restart sequence shows "Camera resources released" followed by a clean start.
-   **Hardware State**:
    -   `/dev/video0` is present.
    -   File Descriptor (FD) leaks have been resolved (verified via `cam_leak_test`).
    -   Previous "Device busy" (EBUSY) errors during restart have been eliminated from the logs.

## Fixes Applied
1.  **Graceful Shutdown**: Added `StopCamera()` to `dialtone.go` to ensure the camera handle is released when the application stops/restarts.
2.  **Leak-Safe Initialization**: Reverted to the "Manual Open -> SetFormat" pattern in `camera_linux.go`.
    -   *Finding*: The atomic `device.Open(..., WithPixFormat(...))` method in `go4vl` was found to leak file descriptors when configuration failed, causing the persistent lock.
    -   *Verification*: `cam_leak_test` tool confirmed the manual pattern is safe.

## Discrepancy
-   **System View**: Logs indicate everything is working. Frames are supposedly being captured and served.
-   **User View**: "It still is not working."

## Investigation Hypotheses (What could be wrong?)

### 1. Silent Capture Failures
The capture loop might be running but receiving:
-   **Empty Frames**: The driver returns 0-byte frames.
-   **Corrupted Data**: The MJPEG data is invalid and the browser can't render it.
-   **Timeout**: `cam.GetFrames()` might be blocking indefinitely despite the successful start.

### 2. Network/Transport Issues
-   The HTTP response headers for MJPEG (`multipart/x-mixed-replace`) might be getting mangled or buffered by proxies (though direct Tailscale connection usually avoids this).
-   Bandwidth issues over Tailscale causing frames to drop entirely.

### 3. Browser Compatibility
-   The specific MJPEG format provided by this camera (L01 HD Webcam) might vary from standard expectations (e.g., missing Huffman tables).

## Proposed Next Steps
1.  **Inspect Frame Data**: Modify `camera_linux.go` to log the *size* of captured frames to confirm data is actually flowing.
2.  **Verify Content-Type**: Check the exact headers being sent to the browser.
3.  **Browser Console**: Check the browser's DevTools for network errors or broken image icons.
4.  **Alternative Format**: Try forcing YUYV format and converting to JPEG in software (slower/CPU intensive but more compatible) to rule out MJPEG hardware issues.
