# SSH Mesh Nodes

## Canonical Targets

| Node | SSH User | Tailscale Hostname | Tailscale IP | SSH Port | Status |
|---|---|---|---|---|---|
| WSL node | `user` | `legion-wsl-1.shad-artichoke.ts.net` | `100.120.147.79` | `22` | Passwordless OK |
| Chroma (macOS) | `dev` | `chroma-1.shad-artichoke.ts.net` | `100.80.248.81` | `22` | Passwordless OK |
| Darkmac (macOS) | `tim` | `darkmac.shad-artichoke.ts.net` | `100.102.107.26` | `22` | Passwordless OK |
| Rover (Linux) | `tim` | `rover-1.shad-artichoke.ts.net` | `100.87.103.114` | `22` | Passwordless OK |
| Legion (Windows) | `timca` | `legion.shad-artichoke.ts.net` | `100.70.50.64` | `2223` | Passwordless OK |

## Recommended SSH Commands

- `ssh user@legion-wsl-1.shad-artichoke.ts.net`
- `ssh dev@chroma-1.shad-artichoke.ts.net`
- `ssh tim@darkmac.shad-artichoke.ts.net`
- `ssh tim@rover-1.shad-artichoke.ts.net`
- `ssh -p 2223 timca@legion.shad-artichoke.ts.net`

## Key Used For Mesh

- Public key label: `mesh@dialtone`
- Fingerprint: `SHA256:V3yLeu/lYYoiskhgb9WW8ozi0c1B+vzSiEWd5Cly79Y`

## Windows Note

Windows OpenSSH is configured with:

- `Match Group administrators`
- `AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys`

The mesh key has been added there, and passwordless SSH is working to `timca@legion` on port `2223`.
