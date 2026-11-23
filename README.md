# Dialtone

## 1. Install

Linux / macOS:

```sh
sh -c 's=$(curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh) && eval "$s"'
```

Windows (PowerShell):

```powershell
$s = curl.exe -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 | Out-String; if ($LASTEXITCODE) { throw 'install.ps1 download failed' }; iex $s
```

Verifies SHA-256 and places only the binary in `~/.local/bin` (`$HOME\.local\bin` on Windows) without editing `PATH` or starting a node. See [`docs/install.md`](docs/install.md) for platforms, options, and source builds.

## 2. CLI (`dialtone --help`)

```text
usage: dialtone [action] [flags]

Single-binary Iroh 1.3 + Rust mesh control plane, 3D tactical HUD, blob store & daemon.
Running `dialtone` with no arguments boots in dependency order and prints the local Web HUD URL.

actions:
  up                       Start the node: HUD -> store -> Iroh QUIC -> mesh
  join <invite>            Join a channel with an invite from any member (dialtone-invite-v2:...),
                           install this binary and start the node at login (--once: only join)
  join <secret>            Unlock (or start) a channel with its secret
  secret                   Generate a new channel secret (dt-...)
  allowlist [list|add|remove] [<node_id>] Manage the allowlist in ~/.dialtone/config.json
  blob [share|pull] <arg>  Share a file or text as a blob, or pull a blob by ticket
  link export <name> --udp|--tcp <ip:port>    Offer a local source to peers over a direct link
  link import <node>/<name> --udp|--tcp <ip:port>  Open a peer's source and deliver it here
  link [list|close <id>]   Show links (path direct or relay, Rtt, packets), or close one
  invite [--ttl 24h]       Make an invite any node can join with until it expires (max 30d)
  daemon [install|status]  install: put this binary in ~/.dialtone/bin, register the login
                           service and start the node (a running one restarts into it)
  probe [--all]            Run one probe now: the gossip checks (MSH, NBR, BEAT; nodes run them
                           every 30s), and with --all the store, pulls, programs and security
                           (STO, PUL, PRC, SEC)
  exec [--node <peer>] [--shell auto|bash|powershell] [--] <cmd...>  Run a bash or PowerShell command on a node
  install <plugin> [--repo <owner/repo>] [--ticket <ticket>] [--file <path>] Install a blob-backed plugin
  plugin [list|share <name>|remove <name>] Manage installed plugins
  shell new <name> [--node <peer>] [--shell auto|bash|powershell] [--cwd <dir>] Start a persistent terminal session
  shell send <name> [--node <peer>] [--timeout 30s] [--] <cmd...> Run a bash/ps1 command in a running session
  shell capture <name> [--node <peer>] [--lines 100] Read scrollback output from a session
  shell ls [--node <peer>] [--json] List running terminal sessions
  shell attach <name> [--node <peer>] Attach interactively to a local or remote session
  shell interrupt <name> [--node <peer>] Interrupt/reset a running session in place (keeps scrollback)
  shell kill <name> [--node <peer>] Stop a running terminal session
  devices [--json]         List the devices connected to this machine (cameras, mics, keyboards, ...)
  status                   Print this node's NodeId and transport report
  help                     Show this help

flags:
  --node <name>            Node name (default: saved config or hostname)
  --listen <addr>          HUD and API listen address (default: 127.0.0.1:8788)
  --api-token <token>      Explicit API bearer token for POST routes (default: ~/.dialtone/api-token)
  --channel <name>         Channel name (default: dialtone)
  --secret <secret>        Channel secret (or set DIALTONE_SECRET)
  --ttl <duration>         TTL for 'dialtone invite' (e.g. 15m, 1h, 7d; default: 24h)
  --shell <shell>          Shell for 'dialtone exec' (auto, bash, powershell; aliases --bash, --powershell)
  --cwd <dir>              Working directory for 'dialtone exec'
  --timeout <duration>     Timeout for 'dialtone exec' / 'dialtone shell send' (e.g. 30s, 2m; default: 30s)
  --ticket <uri>           Blob ticket URI for 'dialtone install <plugin>'
  --repo <owner/repo>      GitHub plugin repo for 'dialtone install <plugin>' (e.g. timcash/dialtone-shell)
  --file <path>            Local blob path for 'dialtone install <plugin>'
  --lines <n>              Number of scrollback lines for 'dialtone shell capture' (default: 100)
  --addr <ip:port>         Direct address for 'dialtone allowlist add' (repeatable)
  --force                  Pull over QUIC even if the blob is already in the store
  --udp <ip:port>          'dialtone link': where the source sends (export) or packets go (import)
  --tcp <ip:port>          'dialtone link': the service to connect to (export) or listen on (import)
  --json                   Print JSON ('devices', 'invite', 'link list', 'exec')
  --once                   'up': boot, beat once and exit; 'join': only join, start nothing
  --all                    'dialtone probe': also test the store, pulls, programs and security

transport (the default: relays on, no public DNS or DHT, Tailscale addresses left out):
  --private-mesh           No relays: dial peers only at their direct addresses
  --relay-only             Advertise only the relay, no direct addresses
  --relay-url <url>        Use this Iroh relay instead of the public ones (repeatable)
  --quic-port <port>       Bind the QUIC endpoint to this UDP port
  --lan                    Also find peers on the local network (mDNS)
  --allow-tailnet          Also use Tailscale addresses
```

## 3. Template Alignment

Keep `dialtone`'s repository layout, single-entry developer script (`./dialtone.sh <action>`), test reporting, and loopback UI aligned with [`timcash/template`](https://github.com/timcash/template):

- One entrypoint per repo (`./dialtone.sh <action>`) owning lint, build, test, dev, and deploy without ad-hoc helper scripts.
- Loopback-only local UI (`127.0.0.1`) reading live state from a single authoritative writer—never a fake fixture or exposed network server.
- As new developer commands or recorded test-UI loops are added, follow `timcash/template`'s action names (`install`, `lint`, `format`, `build`, `test`, `dev`, `clean`, `deploy`) and immutable per-run `logs/state.json` + `logs/<test-id>.{json,log}` contract.

## 4. Test & Live Test Loop

Dialtone has two live test loops: the **self-testing mesh loop** visible in the HUD, and the **repository verification loop** driven by `./dialtone.sh test`.

### Watch the live mesh test loop (HUD + Probes)

1. Start a node (use `DIALTONE_HUD_DIR=src/hud` when editing the HUD so browser reloads pick up changes without rebuilding):

   ```sh
   dialtone up
   # or with live HUD reload:
   DIALTONE_HUD_DIR=src/hud cargo run -- up
   ```

2. Open **`http://127.0.0.1:8788`** in your browser.
3. Every running node automatically runs a **Probe** every 30s (`MSH` mesh membership, `NBR` gossip neighbors, `BEAT` two-way beats) and publishes the result in its **Beat**. Watch live pass/fail checks, neighbor Rtt, store blobs, processes, and active shell sessions update continuously in the HUD (or poll `GET http://127.0.0.1:8788/api/probe` and `GET /api/mesh`).
4. Trigger a full live probe on demand—adding `STO` (store round trip), `PUL` (QUIC blob pulls from peers), `PRC` (supervised probe process), and `SEC` (security checks):

   ```sh
   dialtone probe --all
   ```

### Run the automated repo test loop (`./dialtone.sh test`)

```sh
./dialtone.sh test --here                          # Local & CI loop: leak gate, domain lint, fmt, clippy, cargo test, release build
./dialtone.sh test --here --only lint              # Fast gate: zero-leak check, domain language, rustfmt, clippy
./dialtone.sh test --here --only suite -- --test e2e   # Live 3-node mesh, deploy, link, invite & recovery suite
./dialtone.sh test --here --only installers        # Offline install.sh / install.ps1 fixture tests (Python 3.10+)
./dialtone.sh test --here --only scale --scale 20 --secs 30   # Live multi-node convergence & traffic budget loop
./dialtone.sh test                                 # Full remote VM loop (reads uncommitted fleet.local.env): lint, suite, rejoin, probes, scale
./dialtone.sh test --only probes                   # Verify every remote node's live probe passes
```

On Windows PowerShell, invoke the same runner directly with `cargo run -q -p xtask -- test --here`.

## 5. Domain Language

Every concept in Dialtone has **one word** across CLI, code, API, HUD, and docs (`./dialtone.sh test --here --only lint` rejects banned synonyms in `src/`):

| Word | Meaning |
| --- | --- |
| **Node** | One running `dialtone` binary identified by its persistent Ed25519 **Key** (`NodeId`). |
| **Channel** | A named encrypted gossip room unlocked by a **Secret** (`dt-...`) or joined via an expiring **Invite**. |
| **Mesh** | All **Peers** (other nodes in the channel) permitted by the **Allowlist**. |
| **Beat** | Compact signed status message gossiped periodically (stats, neighbor Rtt, probe checks). |
| **Blob** | Immutable bytes named by a `sha256` **Digest** in the local **Store**, shared by **Ticket**. |
| **Program** | An executable **Blob** staged and run as a supervised **Process** (or rolled out by **Deploy**). |
| **Plugin** | A verified **Blob**-backed extension living in its own repo (e.g. `dialtone-shell`), installed via `dialtone install`. |
| **Session** | A persistent cross-OS terminal (`bash` or `powershell`) managed over QUIC by the `shell` plugin. |
| **Link** | A direct QUIC UDP/TCP tunnel exported by one node and imported by another. |
| **Probe** | A node testing itself and the live mesh (`MSH`, `NBR`, `BEAT`; `--all` adds `STO`, `PUL`, `PRC`, `SEC`). |
| **Daemon** | The OS login service (`launchd`, `systemd --user`, Scheduled Task) that restarts the node on boot or crash. |
| **HUD** | The node's local web UI and bearer-authenticated HTTP **API** on `127.0.0.1:8788`. |

See [`goal.md`](goal.md#domain-language) for the full enforced lexicon and banned-synonym table.

## 6. Docs

- [`docs/install.md`](docs/install.md) — Supported platforms, SHA-256 verification, PATH setup, manual downloads, source builds, and installer tests
- [`docs/guide.md`](docs/guide.md) — User guide, quick start, HUD controls, HTTP API reference, environment variables, and `./dialtone.sh` tasks
- [`docs/plugins.md`](docs/plugins.md) — Blob-backed plugins and cross-OS persistent terminal sessions (`dialtone shell`)
- [`goal.md`](goal.md) — Complete domain language, architecture, security model, and roadmap
