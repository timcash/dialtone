# DAILTONE WSL Agent Runbook

This runbook is for LLM/automation agents managing Dialtone across:

- Windows host: `legion`
- WSL node: `legion-wsl-1`
- Mesh nodes: `darkmac`, `chroma`, `gold`, `rover`

Primary Windows automation account in this runbook: `user` (not `timca`).

## 0. Current Constraints and Known State

- In this session, local Windows PowerShell transport from WSL may fail with:
  - `WSL ... UtilBindVsockAnyPort ... socket failed 1`
- If that happens, run the Windows setup blocks below directly in an **Admin PowerShell** terminal on Legion.
- When bridge/mirror testing requires it, Tailscale on WSL may be intentionally stopped.

## 1. Create Windows Automation User (`user`)

Run on Legion in **Admin PowerShell**:

```powershell
$ErrorActionPreference = "Stop"

if (-not (Get-LocalUser -Name "user" -ErrorAction SilentlyContinue)) {
  $pw = Read-Host 'Password for local user "user"' -AsSecureString
  New-LocalUser -Name "user" -Password $pw -FullName "Dialtone User" -Description "Dialtone automation account"
}

if (-not (Get-LocalGroupMember -Group "Administrators" -Member "user" -ErrorAction SilentlyContinue)) {
  Add-LocalGroupMember -Group "Administrators" -Member "user"
}
```

## 2. Enable Remote Control Surfaces on Windows

Run on Legion in **Admin PowerShell**:

```powershell
$ErrorActionPreference = "Stop"

# PowerShell remoting
Enable-PSRemoting -Force

# Avoid filtered admin token issues for local-admin remoting
New-ItemProperty `
  -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System' `
  -Name LocalAccountTokenFilterPolicy `
  -PropertyType DWord `
  -Value 1 `
  -Force | Out-Null

# OpenSSH server
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Set-Service sshd -StartupType Automatic
Start-Service sshd

# SSH firewall
if (-not (Get-NetFirewallRule -Name "sshd-In-TCP" -ErrorAction SilentlyContinue)) {
  New-NetFirewallRule -Name "sshd-In-TCP" -DisplayName "OpenSSH Server (sshd)" -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22
}
```

## 3. Install SSH Keys for `user`

### 3.1 Windows inbound key auth (`user@legion`)

Run on Legion in **Admin PowerShell**:

```powershell
$sshDir = "C:\Users\user\.ssh"
$auth = Join-Path $sshDir "authorized_keys"

New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
if (-not (Test-Path $auth)) { New-Item -ItemType File -Path $auth -Force | Out-Null }

# Permissions
icacls $sshDir /inheritance:r | Out-Null
icacls $sshDir /grant "user:(OI)(CI)F" | Out-Null
icacls $auth /inheritance:r | Out-Null
icacls $auth /grant "user:F" | Out-Null
```

Append the agent public key(s) to `C:\Users\user\.ssh\authorized_keys`.

Example key format:

```text
ssh-ed25519 AAAA... comment
```

### 3.2 Mesh key on other nodes (`gold`, etc.)

For any node that should both accept and initiate mesh SSH:

- Put mesh private key in `~/.ssh/id_ed25519_mesh` (`0600`)
- Put mesh public key in `~/.ssh/id_ed25519_mesh.pub` (`0644`)
- Add mesh public key to `~/.ssh/authorized_keys` (`0600`)

## 4. WSL Networking Mode

### 4.1 Mirrored mode (preferred for LAN-style testing)

Set on Windows host (file: `%UserProfile%\.wslconfig`):

```ini
[wsl2]
networkingMode=mirrored
dnsTunneling=true
autoProxy=true
```

Apply:

```powershell
wsl --shutdown
```

### 4.2 Bridge mode

If using bridge mode, ensure:

- Expected LAN reachability exists
- Firewall rules are aligned
- Agent understands Tailscale may be disabled during bridge-only tests

## 5. Tailscale Policy for Testing

### 5.1 Disable on WSL (bridge-only scenarios)

```bash
sudo tailscale down || true
sudo systemctl stop tailscaled || true
```

### 5.2 Re-enable when tailnet access is required

```bash
sudo systemctl start tailscaled
tailscale status
```

If re-auth needed:

```bash
sudo tailscale up --auth-key <tskey-auth-...> --accept-routes --ssh --hostname=legion-wsl
```

## 6. Chrome Remote Service Architecture

Preferred pattern:

1. Deploy chrome binary to remote node
2. Start remote `service-daemon` on port `19444`
3. Request session via `debug-url --host <node>`
4. Use returned service-proxied websocket URL for chromedp

Examples:

```bash
./dialtone.sh chrome src_v1 deploy --host darkmac --service --role dev
./dialtone.sh chrome src_v1 debug-url --host darkmac --role dev --service-port 19444
```

Expected URL shape:

```text
ws://<node-host>:19444/devtools/browser/<id>
```

## 7. Mesh Node Standardization

Use canonical users in mesh config:

- `legion`: `user` (target state)
- `gold`: `user`
- `darkmac`: `tim`
- `chroma`: `dev`
- `rover`: `tim`

## 8. Validation Checklist (Agent)

Before running plugin tests:

1. `./dialtone.sh ssh src_v1 mesh` shows expected nodes/users/transports
2. Passwordless SSH works for target node(s)
3. WSL networking mode is known (`mirrored` or `bridge`)
4. Chrome service daemon is running on remote if using host mode
5. Use `./dialtone.sh ... test` (not raw `go test` unless explicitly requested)

## 9. Validation Commands

From WSL:

```bash
./dialtone.sh ssh src_v1 mesh
./dialtone.sh ssh src_v1 run --node gold --cmd "whoami"
./dialtone.sh chrome src_v1 remote-doctor --nodes darkmac,legion --ports 9333,19333
./dialtone.sh chrome src_v1 debug-url --host darkmac --role dev --service-port 19444
```

On Windows host:

```powershell
Get-LocalUser user
Get-LocalGroupMember Administrators | Where-Object Name -Match "user"
Get-Service sshd
wsl --status
```

## 10. Troubleshooting

- `ssh ... no supported methods remain`
  - wrong username for node
  - missing key in `authorized_keys`
  - bad key file permissions

- Windows admin command fails with `Access is denied`
  - run from elevated PowerShell
  - use scheduled-task elevation bridge if fully non-interactive elevation is required

- `debug-url --host ...` falls back to localhost tunnel
  - remote daemon not reachable on `19444`
  - old loopback-only browser instance reused
  - verify remote process args include `--remote-debugging-address=0.0.0.0`

- WSL cannot resolve external DNS/API
  - validate mirrored mode + `dnsTunneling=true`
  - restart WSL (`wsl --shutdown`)
  - verify resolver and connectivity

- Reverse mesh SSH from non-tailnet node fails
  - node missing Tailscale and/or MagicDNS
  - use LAN hop/jump host until node joins tailnet

## 11. Security Notes

- Do not commit passwords or auth keys to repo.
- Prefer key-based auth everywhere.
- Minimize permanent admin exposure.
- Rotate temporary auth keys regularly.

