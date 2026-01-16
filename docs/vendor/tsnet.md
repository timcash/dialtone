# Networking (Tailscale & tsnet)

Dialtone leverages a modern, identity-based networking stack to eliminate the need for port forwarding or public IPs.

## tsnet (Tailscale)

The system embeds Tailscale directly via `tsnet`. It appears as a first-class node on your private **tailnet**, providing automatic wireguard encryption and stable DNS (MagicDNS).

## Automated Tailscale Provisioning

Dialtone uses two types of Tailscale credentials to manage its secure, per-process VPN network without requiring a system-level installation:

1.  **Tailscale API Access Token**: (Conceptual "Master Key") Used only on your local machine to programmatically generate smaller "Visitor Keys".
    - **How to get it**: Go to [Tailscale Settings > Keys](https://login.tailscale.com/admin/settings/keys) and generate an "Access Token".
2.  **Tailscale Auth Key**: (Conceptual "Ephemeral Visitor Key") A temporary, short-lived credential that allows the `dialtone` process on the robot to join your network.
    - **How it works**: When you run `dialtone provision`, the CLI uses your API Token to request a one-time use, ephemeral key from Tailscale.

### Key Propagation & Security

Security is maintained by ensuring the Auth Key is never permanently stored on the robot's disk:
1.  **Local Storage**: The `provision` command saves the generated `TS_AUTHKEY` in your local `.env` file.
2.  **SSH Propagation**: When you run `dialtone deploy`, the local CLI reads the key from `.env` and passes it to the remote computer via the **SSH environment**. 
3.  **Process Injection**: The remote `dialtone` binary is started via an `env TS_AUTHKEY=...` command inside a `nohup` block. The key exists only in the volatile memory of the running process.
4.  **Auto-Cleanup**: Because the key is marked as **ephemeral**, Tailscale will automatically remove the robot from your machine list as soon as the process disconnects, keeping your admin console clean.

### Provision Tailscale Key

If you have a Tailscale API Access Token, you can generate a new auth key and update `.env` automatically:
```bash
bin/dialtone.exe provision -api-key your_tailscale_api_token
```
