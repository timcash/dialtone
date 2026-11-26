# Hot-Loadable Dialtone Plugins

This document specifies how Dialtone plugins are discovered, verified, invoked, installed, updated, and removed **without restarting the running `dialtone up` daemon**.

## Background: How Dialtone Works Today

- `dialtone` is a single Rust binary installed at `~/.dialtone/bin/dialtone` (or `%USERPROFILE%\.dialtone\bin\dialtone.exe` on Windows) that acts as both the CLI front-end and the long-running mesh daemon (`dialtone up`).
- CLI commands like `dialtone exec`, `dialtone install`, and `dialtone plugin list|share|remove` talk to the running daemon over its local HTTP API at `http://127.0.0.1:8788`, authenticating `POST` requests with the bearer token in `~/.dialtone/api-token` (`0600`).
- Installed plugins record a `dialtone.plugin.v1` manifest at `~/.dialtone/plugins/<name>/manifest.json` containing `kind` (e.g. `blob-binary`, `builtin-protocol`), `blob_digest` (`sha256:<64 hex>`), `blake3_hash`, and `bytes`.
- Previously, passing an unknown subcommand such as `dialtone task list` fell through `parse_cli` in `src/main.rs` to `Err("unknown argument \"task\"")`, printed full usage text, and exited with status code `2`.

---

## (a) Call-Time CLI Subcommand Dispatch

When invoked as `dialtone [global-flags...] <name> [plugin-args...]`, the CLI resolves `<name>` at call time, similar to `git <subcommand>` or `kubectl <plugin>`:

1. **Built-in commands always win.**
   The built-in command table (`up`, `join`, `secret`, `allowlist`, `blob`, `link`, `invite`, `daemon`, `probe`, `exec`, `install`, `plugin`, `shell`, `devices`, `status`, `help`, plus `-h`/`--help`) is matched before plugin dispatch. Even if a plugin directory named `status` or `exec` exists under `~/.dialtone/plugins/`, the built-in command always executes and the plugin dispatch layer refuses any built-in name as defense in depth.

2. **Discovery without daemon lock contention.**
   When the first non-flag positional token is not a built-in command, `parse_cli` stops parsing flags immediately and captures `<name>` and all remaining arguments verbatim as `plugin_args` (so plugin flags like `--help`, `--json`, `--node`, or `--` pass through untouched).
   The CLI reads `<state_dir>/plugins/<name>/manifest.json` directly from disk (`$DIALTONE_STATE_DIR/plugins` or `~/.dialtone/plugins`). It does **not** open `~/.dialtone/blobs`, avoiding contention on `store.lock` held by the running `dialtone up` daemon.

3. **Executable resolution inside `~/.dialtone/plugins/<name>/`.**
   Only executable plugin kinds (not `builtin-protocol`) are dispatched. Candidate binary paths are checked in order inside `<plugins_dir>/<name>/`:
   1. Optional `manifest.entrypoint` (validated as a safe relative path with no `..`, root, or drive prefix; on Windows also checks `<entrypoint>.exe` if `.exe` is omitted).
   2. Platform default binary name: `dialtone-<name><exe_suffix>` (e.g. `dialtone-task` on macOS/Linux, `dialtone-task.exe` on Windows).
   3. Short name fallback: `<name><exe_suffix>`.
   4. Staged blob-store path: `bin/dialtone-<name>-<short12><exe_suffix>` where `<short12>` is the first 12 hex characters of `blob_digest`.

4. **Fail-closed digest verification before execution.**
   Before spawning the binary, the CLI normalizes `manifest.blob_digest` (`sha256:<64 hex>`), streams the candidate binary file through SHA-256 while counting bytes, and compares both the observed SHA-256 digest and byte length (`manifest.bytes`, when non-zero) against the manifest. Any mismatch or unreadable file aborts execution with a clear refusal message and non-zero exit code.

5. **Process invocation, stdio, args, env, and exit code.**
   - **Arguments:** all tokens after `<name>` are forwarded unchanged in `argv[1..]`.
   - **Stdio:** `stdin`, `stdout`, and `stderr` are inherited directly so interactive prompts, piping, ANSI output, and JSON streaming work natively.
   - **Exit code:** the child process exit code is returned as `dialtone`'s own exit code (on Unix, termination by signal `N` maps to `128 + N`).
   - **Environment variables passed to the plugin:**
     - `DIALTONE_HOME`: resolved Dialtone state directory (e.g. `/Users/tim.c/.dialtone`).
     - `DIALTONE_STATE_DIR`: same resolved state directory (matches `dialtone::mesh::default_state_dir()`).
     - `DIALTONE_API`: local daemon HTTP base URL (e.g. `http://127.0.0.1:8788`).
     - `DIALTONE_NODE`: local node name (from `--node`, `config.json`, or hostname).
     - `DIALTONE_PLUGIN_NAME`: canonical plugin name (e.g. `task`).
     - `DIALTONE_PLUGIN_DIR`: plugin directory (`<state_dir>/plugins/<name>`).
   - **Secret hygiene:** secrets (`api-token`, `secret_key`, `iroh-secret-key`, `gateway_token`, `mesh.key`, `rpc.token`, tickets, or invites) are **never** placed in `argv` (which is visible via `ps`). Sensitive node-only environment variables (`DIALTONE_SECRET`, `DIALTONE_SECRET_KEY`, `DIALTONE_GATEWAY_TOKEN`, `MESH_ORIGIN_TOKEN`) are removed from the spawned plugin environment. A trusted local plugin that needs to call an authenticated local daemon `POST` endpoint reads `$DIALTONE_HOME/api-token` directly from disk (`0600`).

6. **Not-found error UX.**
   If `<name>` is not a valid plugin name or has no valid manifest under `<plugins_dir>/`, `dialtone` prints an error to `stderr` listing currently installed plugins (for example: `dialtone: unknown command 'foo' (installed plugins: shell, task)`) and exits with code `2`.

---

## (b) Backwards-Compatible Manifest Extension (`dialtone.plugin.v1`)

Existing `dialtone.plugin.v1` manifests (such as `task` and `shell`) remain valid with no migration required. New fields are optional (`#[serde(default, skip_serializing_if = "Option::is_none")]`):

- `entrypoint` (optional string): relative path to the plugin binary inside `~/.dialtone/plugins/<name>/` (e.g. `"dialtone-task"` or `"bin/dialtone-task-04db5e8141d8"`). Must stay strictly inside the plugin directory.
- `binaries` (optional map of `<os>-<arch>` -> object): describes per-platform executable names and digests for multi-platform plugin distribution (`macos-arm64`, `linux-x86_64`, `linux-arm64`, `windows-x86_64`).
- `service` (optional object): describes a long-running background process shipped by the plugin (such as `dialtone-task serve`):
  - `args`: command-line arguments to start the service (supports `${DIALTONE_NODE}` expansion).
  - `autostart`: boolean indicating whether the service should start automatically on login/boot or daemon reconcile.
  - `restart`: restart policy (`"on-failure"`, `"always"`, or `"never"`).
  - `rpc_addr`: optional loopback RPC address (e.g. `"127.0.0.1:8794"`).
  - `health_path`: optional HTTP health path on `rpc_addr` (e.g. `"/health"`).
  - `stop_grace_secs`: seconds to wait after SIGTERM/stop before force-killing during an upgrade or removal.
  - `os_service`: optional per-OS external unit names (`launchd`, `systemd_user`, `windows_task`) used until a daemon-side supervisor takes ownership.

Example extended manifest for `task`:

```json
{
  "schema": "dialtone.plugin.v1",
  "name": "task",
  "version": "0.1.0",
  "kind": "blob-binary",
  "description": "Shared CRDT task tracker over Dialtone metal mesh",
  "protocols": ["dialtone/task/1"],
  "blob_digest": "sha256:04db5e8141d889e7332d8b732f76e67a0f9dd2c81f00957c1495c90e77c49b96",
  "blake3_hash": "001bece3f4a25a15398f486eb9b71d51e7af5f47a33b26b3c851b4a9fd5f2710",
  "bytes": 13919056,
  "entrypoint": "dialtone-task",
  "binaries": {
    "macos-arm64": { "filename": "dialtone-task" },
    "linux-x86_64": { "filename": "dialtone-task" },
    "linux-arm64": { "filename": "dialtone-task" },
    "windows-x86_64": { "filename": "dialtone-task.exe" }
  },
  "service": {
    "args": ["serve", "--node", "${DIALTONE_NODE}"],
    "autostart": true,
    "restart": "on-failure",
    "rpc_addr": "127.0.0.1:8794",
    "stop_grace_secs": 5,
    "os_service": {
      "launchd": "earth.dialtone.task",
      "systemd_user": "dialtone-task.service",
      "windows_task": "DialtoneTask"
    }
  },
  "installed_at_unix": 1795725515
}
```

---

## (c) Hot Install, Update, and Remove Without Daemon Restart

Because plugins live in `~/.dialtone/plugins/<name>/` and run as independent processes, installing, updating, or removing a plugin **never restarts `dialtone up`**:

1. **Hot install (`dialtone install <name> --file|--ticket|--repo`):**
   - The CLI sends `POST /api/plugins/install` to the already-running local daemon (so the running daemon writes to its `iroh-blobs` store without `store.lock` conflict).
   - For `blob-binary` plugins, the installer materializes the verified executable into `~/.dialtone/plugins/<name>/` by writing a temp file in the same directory, verifying its SHA-256 digest, setting mode `0755` on Unix, and atomically renaming it into place, followed by an atomic rename of `manifest.json.tmp.<pid>` to `manifest.json`.
   - The next `dialtone <name> ...` invocation immediately discovers and runs the new plugin.

2. **Hot update (replacing a plugin binary in place):**
   - **Zero-race content-addressed layout (recommended for future updates):** stage the new binary to `bin/dialtone-<name>-<short12><exe_suffix>`, set `manifest.entrypoint` to that relative path, and atomically rename `manifest.json.tmp` over `manifest.json`. Concurrent CLI calls either read the old manifest + old binary or the new manifest + new binary, with zero mismatch window.
   - **Fixed-path layout (when an external OS unit runs `dialtone-<name>` directly):**
     - **macOS / Linux:** write the verified new binary to `.dialtone-<name>.new.<pid>` in `~/.dialtone/plugins/<name>/`, `chmod 0755`, atomically `rename` it over `dialtone-<name>`, atomically update `manifest.json`, and then restart **only the plugin's own service** (e.g. `launchctl kickstart -k gui/$UID/earth.dialtone.task` or `systemctl --user restart dialtone-task.service`).
     - **Windows:** stop only the plugin's own service/process (e.g. `DialtoneTask`), or rename the running `dialtone-<name>.exe` to `dialtone-<name>.prev.exe`, rename the staged `dialtone-<name>.new.exe` to `dialtone-<name>.exe`, atomically update `manifest.json`, and start only the plugin's own service.
   - In all cases, the base `dialtone up` daemon PID and mesh connections are untouched.

3. **Hot remove (`dialtone plugin remove <name>`):**
   - Stop only the plugin's service/process if `manifest.service` is active (never the base daemon).
   - Remove the plugin's manifest and executable (optionally preserving plugin data files if a future `--keep-data` flag is passed; by default `remove_installed_plugin` removes `~/.dialtone/plugins/<name>/`).
   - Subsequent `dialtone <name> ...` calls immediately report `<name>` as not installed.

---

## (d) Local API Discovery & Future Daemon-Side Plugin Supervision

### Daemon Discovery Today (Zero Daemon Changes)

Dispatched plugin CLIs and plugin `serve` processes can discover and talk to the local daemon using its existing HTTP API at `DIALTONE_API` (default `http://127.0.0.1:8788`):

- Read-only discovery routes (no auth required on loopback):
  - `GET /api/mesh`: live peer list (names, NodeIds, advertised direct IPs and relay URLs, OS/arch, RTT).
  - `GET /api/plugins`: installed plugins and verification status.
  - `GET /api/state`, `GET /api/probe`, `GET /api/links`, `GET /api/processes`.
- Authenticated local actions (`Authorization: Bearer <token>` read from `$DIALTONE_HOME/api-token`):
  - `POST /api/blob/share` and `POST /api/blob/pull` to exchange verified blobs over the base mesh.

### Future Step: Daemon-Side Plugin Process Supervisor

Today, long-running plugin daemons such as `dialtone-task serve` use per-OS units (`earth.dialtone.task` LaunchAgent, `dialtone-task.service` systemd user unit, `DialtoneTask` Scheduled Task). As a future step:

- The base daemon already has `dialtone::supervisor::Supervisor` (`src/supervisor.rs`), which manages `ProcessSpec` entries with Unix process-group isolation, stale-tree reaping, and Windows kill-on-close Job Objects, and stops/restarts a single child automatically whenever its `path` or `digest` changes.
- Extending `POST /api/plugins/install` and `POST /api/plugins/remove` (plus a `POST /api/plugins/restart` endpoint) to reconcile `manifest.service` entries into `Supervisor` would let the base daemon supervise plugin `serve` processes uniformly across macOS, Linux, Windows, and containers without per-plugin OS units.
- Note: `Supervisor::spawn` currently strips all `DIALTONE_*` environment variables (`src/supervisor.rs:637`) to protect `DIALTONE_SECRET`. A future plugin supervisor should continue stripping secrets (`DIALTONE_SECRET*`, `DIALTONE_GATEWAY_TOKEN`, `MESH_ORIGIN_TOKEN`) while passing the safe allowlist (`DIALTONE_HOME`, `DIALTONE_STATE_DIR`, `DIALTONE_API`, `DIALTONE_NODE`, `DIALTONE_PLUGIN_NAME`, `DIALTONE_PLUGIN_DIR`).

---

## (e) Does Enabling This Require a One-Time Daemon Upgrade or Restart?

**Short answer:** Enabling call-time CLI plugin dispatch requires a **one-time replacement of the `dialtone` binary file on disk** (`~/.dialtone/bin/dialtone` or `%USERPROFILE%\.dialtone\bin\dialtone.exe`), and **does NOT require restarting the running `dialtone up` daemon**.

### Why No Daemon Restart Is Needed

1. **CLI dispatch runs purely in the CLI process.**
   `dialtone` is one binary used for both `dialtone up` (daemon) and short-lived CLI commands. Call-time plugin dispatch happens entirely inside the new CLI process (reading `~/.dialtone/plugins/<name>/manifest.json`, verifying the plugin binary's SHA-256 on disk, and spawning it). It uses zero new daemon HTTP endpoints; existing plugin management commands (`install`, `plugin list|share|remove`) already work against the currently running daemon.
2. **Replacing the binary file on disk leaves the running daemon process untouched:**
   - **macOS and Linux:** writing the new binary to a temporary file in `~/.dialtone/bin/` (e.g. `dialtone.new`), setting `0755`, and atomically renaming (`mv`) it over `~/.dialtone/bin/dialtone` replaces only the directory entry. The running `dialtone up` process continues executing from its already-mapped old inode with the same PID and open QUIC connections. Every new CLI invocation executes the new inode immediately.
   - **Windows:** a running `.exe` cannot be overwritten in place, but NTFS allows renaming a running `.exe` inside the same directory (the exact mechanism `src/deploy.rs` and `src/update.rs` already use). Renaming `dialtone.exe` to `dialtone.prev.exe` and then renaming `dialtone.new.exe` to `dialtone.exe` leaves the running daemon process undisturbed while all subsequent CLI invocations run the new `dialtone.exe`.

### Audit of Login Services, Update Loops, and Watchdogs

We audited every supervisor, service definition, updater, and watchdog in the codebase and on `rosie` to confirm whether any of them notice the binary change on disk and trigger a daemon restart:

| Component | Behavior when `~/.dialtone/bin/dialtone` is replaced on disk | Triggers restart? |
| --- | --- | --- |
| macOS LaunchAgent (`com.timcash.dialtone.plist`) | Uses `RunAtLoad=true`, `KeepAlive=true`. No `WatchPaths` or `QueueDirectories`. | **No** |
| Linux systemd user unit (`dialtone.service`) | Uses `ExecStart=...`, `Restart=always`, `RestartSec=2`. No `.path` unit or file watch. | **No** |
| Windows Scheduled Task (`start-dialtone.cmd`) | Simple `:loop` blocking on `dialtone.exe up ...`. Does not inspect binary mtime or hash. | **No** |
| Container `run-loop.sh` / `supervisor::keep_running` | Waits on child exit (`child.wait()`). Does not watch binary file on disk. | **No** |
| Watchdog thread (`actors::spawn_watchdog`) | Checks only in-memory gossip `NetCounters` (`mesh::stuck_reason`). Never reads `bin/dialtone`. | **No** |
| Allowlist actor (`actors::allowlist_actor`) | Polls only `config.json` mtime. Never touches `bin/dialtone`. | **No** |
| Self-update actor (`actors::update_actor`, `~/.dialtone/update/`, `~/.dialtone/update.json`) | `running_version()` is latched at startup (`OnceLock`). `share_own_binary` returns early if `state.shared` already matches the release manifest, or silently skips if the on-disk hash does not match the signed manifest. It **only** installs and calls `request_restart()` when a **newer signed GitHub release** (higher semver than `running_version()`, signed by `RELEASE_PUBLIC_KEY`) is published or announced by a peer. | **No** (for same-version binary replacement; see caveats below) |

### Operational Caveats

1. **Do NOT run `dialtone daemon install` for a zero-restart rollout.** `dialtone daemon install` calls `install_and_start()`, which deliberately restarts the running daemon (`launchctl bootout/bootstrap`, `systemctl --user restart`, or `POST /api/restart`). Replace only the binary file in `~/.dialtone/bin/`.
2. **Interaction with future signed releases (`DIALTONE_AUTO_UPDATE`).** Because `update_actor` checks GitHub every hour (`UPDATE_CHECK_EVERY = 3600s`) and listens for newer peer versions, publishing a newer signed release (e.g. `v0.5.6`) will cause auto-update to download that signed release, move the manually placed binary to `dialtone.prev`, and restart the daemon into `v0.5.6`. Merge `feat/plugin-dispatch` before cutting the next signed release so auto-update preserves CLI dispatch.
3. **Behavior on next natural daemon restart (reboot or crash).** At startup, `update::on_start` hashes `paths.exe`. If its SHA-256 differs from `update.json`'s `committed.sha256` and `trial` is `null` (or mismatched, which `on_start` explicitly clears as "replaced by hand"), the daemon boots cleanly with `CARGO_PKG_VERSION` and does **not** roll back to `dialtone.prev`.
4. **Future daemon-side features.** Only future features that run inside the daemon (such as a daemon-side plugin process supervisor or new `/api/plugins/*` routes) will require a daemon restart when adopted. All future plugin installs, updates, removals, and CLI invocations will work hot without ever restarting `dialtone up`.
