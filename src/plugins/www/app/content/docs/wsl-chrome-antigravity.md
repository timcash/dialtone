# Antigravity Sub-agent on WSL

This guide explains how to set up the Antigravity browser sub-agent to run from Windows Subsystem for Linux (WSL) and interact with Chrome on the Windows host.

## Prerequisites

1.  **WSL2**: Ensure you are running WSL2.
2.  **Chrome on Windows**: Google Chrome installed on the Windows host.

## Step 1: Launch Chrome with Remote Debugging

The sub-agent communicates with Chrome via the DevTools protocol. You must launch Chrome on Windows with the remote debugging port enabled.

From a Windows command prompt or PowerShell:
```powershell
start chrome --remote-debugging-port=9222
```

## Step 2: Install the Antigravity Extension

The sub-agent requires a specific extension to facilitate interaction.

1.  Open Chrome on Windows.
2.  Navigate to the [Antigravity Browser Extension](https://chromewebstore.google.com/detail/antigravity-browser-exten/eeijfnjmjelapkebgockoeaadonbchdd).
3.  Click **Add to Chrome**.
4.  If you encounter a "Download Error," try refreshing the page or restarting Chrome.

## Step 3: Configure WSL Networking

WSL needs to reach `localhost:9222` on the Windows host.

### Option A: Mirrored Networking (Recommended)
This is the simplest method as it shares the host's IP addresses with WSL.

1.  Open `%userprofile%\.wslconfig` in Windows.
2.  Add the following:
    ```ini
    [wsl2]
    networkingMode=mirrored
    ```
3.  Restart WSL: `wsl --shutdown` then reopen your terminal.

### Option B: Port Forwarding
If mirrored networking doesn't work, you can bridge the connection.

1.  Identify the host IP from WSL: `ip route show | grep default | awk '{print $3}'`
2.  Use a tool like `socat` to bridge `127.0.0.1:9222` in WSL to the Host IP's port `9222`.

## Step 4: Verification

In WSL, you can verify the connection by running:
```bash
curl -s http://127.0.0.1:9222/json/version
```
If successful, you will see a JSON response containing the Chrome version and `webSocketDebuggerUrl`.

## Troubleshooting

- **Connection Refused**: Ensure Chrome is actually running with the `--remote-debugging-port=9222` flag.
- **Extension Not Detected**: Verify the extension is enabled in `chrome://extensions/`.
