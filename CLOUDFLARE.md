# Cloudflare Plugin Updates

# Log 2026-02-14 18:30
- **Fixed Compilation Errors:** 
    - Added `CloudflaredVersion` constant to `src/core/install/install.go`.
    - Added missing `os/exec` import to `src/dialtone.go` for `checkZombieProcess`.
- **Provisioned "test" Tunnel:** Created a new Cloudflare tunnel named "test" and successfully configured the CNAME for `test.dialtone.earth`.
- **CLI Bug Fix:** Fixed `dialtone cloudflare tunnel run` to correctly handle the `--token` flag by excluding the tunnel name argument when a token is provided (Cloudflare requirement).
- **Backgrounded Tunnel:** Successfully started and backgrounded the Cloudflare tunnel for the "test" subdomain, targeting `http://127.0.0.1:8080`.
- **Env Configuration:** Automatically saved the new `CF_TUNNEL_TOKEN_TEST` to `env/.env`.

This document outlines the current architecture and steps for setting up a Cloudflare Tunnel to expose the `dialtone` application running on a remote robot to the internet via `dialtone.earth`.

## Architecture: Robot -> Tailscale -> Local Relay -> Cloudflare Tunnel -> Internet

Contrary to an initial approach of running `cloudflared` directly on the remote robot, the intended architecture leverages *this local computer* as a relay.

1.  **Robot (`dialtone` application):** Runs the main `dialtone` application, serving its web UI and NATS over Tailscale. It does *not* run `cloudflared`.
2.  **Tailscale Network:** Provides a secure, private network connection between the remote robot and this local computer.
3.  **Local Relay (This Computer):** Runs `dialtone vpn` to access the robot's services over Tailscale and exposes them locally (e.g., `http://127.0.0.1:8080`). This computer then also runs `cloudflared` to establish the Cloudflare Tunnel.
4.  **Cloudflare Tunnel:** Connects the local relay's exposed service (`http://127.0.0.1:8080`) to Cloudflare's edge network.
5.  **Internet:** Users access the `dialtone` UI via a `dialtone.earth` subdomain, which is routed through the Cloudflare Tunnel.

## Non-Interactive Cloudflare Tunnel Setup using Service Tokens

To streamline the tunnel setup and avoid interactive browser logins, the `dialtone` CLI now supports using a Cloudflare Tunnel Service Token.

**Key Change:** The `CF_TUNNEL_TOKEN` environment variable has been introduced. When set, `dialtone cloudflare tunnel run` will use this token for authentication, allowing for a fully non-interactive setup once the token is configured.

## Step-by-Step Setup Guide

Follow these steps on your **local computer** unless specified otherwise:

1.  **Generate a Cloudflare Tunnel Service Token:**
    *   Go to your Cloudflare Dashboard.
    *   Navigate to "Access" -> "Tunnels".
    *   Select your tunnel (or create one if you haven't already).
    *   Go to "Configure" -> "Token".
    *   Generate a new "service token" (starts with `eyJh...`). Copy this token.

2.  **Add the service token to your local `env/.env` file:**
    Open your `env/.env` file and add/update the following line:
    ```
    CF_TUNNEL_TOKEN=your-cloudflare-tunnel-service-token
    ```
    *(Replace `your-cloudflare-tunnel-service-token` with the token you copied.)*

3.  **Ensure `cloudflared` is installed locally:**
    Run:
    ```bash
    ./dialtone.sh install
    ```

4.  **Start `dialtone` on the *robot*:**
    SSH into the remote robot and run:
    ```bash
    sudo /home/tim/dialtone_deploy/dialtone start
    ```
    *(Ensure `/home/tim/dialtone_deploy/dialtone` is the correct path on your robot.)*

5.  **Start the local `dialtone vpn` relay on *this computer*:**
    Run:
    ```bash
    ./dialtone.sh vpn
    ```
    *(Optional: Add `--hostname drone-1` if you want to explicitly name your local `tsnet` client.)*
    This will expose the robot's `dialtone` UI on `http://127.0.0.1:8080` locally.

6.  **Create a Cloudflare Tunnel (if you haven't already):**
    Choose a `<tunnel-name>` (e.g., `drone-tunnel`) and run:
    ```bash
    ./dialtone.sh cloudflare tunnel create <tunnel-name>
    ```

7.  **Route the subdomain to your tunnel:**
    Choose your desired subdomain (e.g., `drone-1.dialtone.earth`). If omitted, it defaults to `DIALTONE_HOSTNAME.dialtone.earth`.
    ```bash
    ./dialtone.sh cloudflare tunnel route <tunnel-name> drone-1.dialtone.earth
    ```

8.  **Run the Cloudflare Tunnel:**
    Use the tunnel name from step 6. The `CF_TUNNEL_TOKEN` from your `env/.env` will be used automatically.
    ```bash
    ./dialtone.sh cloudflare tunnel run <tunnel-name> --url http://127.0.0.1:8080
    ```

After these steps, your robot's `dialtone` UI should be accessible via `https://drone-1.dialtone.earth`.

## Next Steps / Suggestions

*   **Verification:** Confirm accessibility of the `dialtone` UI via the configured `dialtone.earth` subdomain.
*   **Persistent Tunnel:** Consider setting up the `cloudflared tunnel run` command as a persistent service (e.g., using `systemd` or `nohup`) on your local relay machine if you want the tunnel to remain active after you close your terminal.
*   **Documentation for `dialtone.sh cloudflare`:** Enhance the `README.md` in `src/plugins/cloudflare` with explicit examples for `CF_TUNNEL_TOKEN` usage.
*   **Error Handling:** Improve error messages and provide more specific guidance if tunnel creation or routing fails.