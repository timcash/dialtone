# Plugins & Persistent Shell Sessions

Dialtone plugins are content-addressed programs stored in the node's `iroh-blobs` **Store** (`~/.dialtone/blobs`), verified by SHA-256 **Digest**, and installed per-repo or via peer **Blob** **Ticket**.

Each plugin lives in its own repository (for example, `timcash/dialtone-shell`) so the core `dialtone` binary stays small and plugins ship independently.

## Installing & Managing Plugins

```sh
# Install a plugin from its repository (default repo for `shell`: timcash/dialtone-shell)
dialtone install shell
dialtone install <plugin> --repo <owner/repo>

# Or install directly from a peer blob ticket or local file
dialtone install <plugin> --ticket <iroh-blobs-ticket>
dialtone install <plugin> --file <path>

# Inspect, share over the mesh, or remove installed plugins
dialtone plugin list [--json]
dialtone plugin share <plugin>
dialtone plugin remove <plugin>
```

When installed, the plugin binary is imported into the local `Store`, recorded with its verified SHA-256 `Digest`, and staged on demand as a supervised `Process` when invoked.

## Cross-OS Persistent Shell Sessions (`dialtone shell`)

The `shell` plugin manages long-running, stateful `bash` or `powershell` **Sessions** on the local **Node** or any allowlisted **Peer** over Dialtone QUIC (`dialtone/shell/1`), without SSH or `tmux`:

```sh
# Start a named persistent session (local or on any peer with --node <peer>)
dialtone shell new <name> [--node <peer>] [--shell auto|bash|powershell] [--cwd <dir>]

# Send a command to the running session and wait for its marker/exit code
dialtone shell send <name> [--node <peer>] [--timeout 30s] [--] <command...>

# Read scrollback output from a session
dialtone shell capture <name> [--node <peer>] [--lines 100]

# List running sessions
dialtone shell ls [--node <peer>] [--json]

# Attach interactively to a local or remote session
dialtone shell attach <name> [--node <peer>]

# Interrupt/reset a session in place while preserving scrollback
dialtone shell interrupt <name> [--node <peer>]

# Stop and remove a session
dialtone shell kill <name> [--node <peer>]
```

Active plugins and running sessions also appear in the local **HUD** (`http://127.0.0.1:8788`) and at `GET /api/shell/sessions`.
