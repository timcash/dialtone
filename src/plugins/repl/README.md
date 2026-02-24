# REPL Plugin

The REPL plugin provides:
- local interactive REPL (`run`)
- shared multi-client REPL over NATS (`serve`/`join`)
- update-aware service supervisor and OS persistence (`service`)
- release build/publish tooling for per-architecture binaries (`release`)
- environment discovery and bus health checks (`status`)

Prompts default to host identity (`<hostname>`) instead of `USER-1`.

## CLI
```bash
./dialtone.sh repl src_v1 help
./dialtone.sh repl src_v1 run
./dialtone.sh repl src_v1 status
./dialtone.sh repl src_v1 serve --nats-url nats://0.0.0.0:4222 --room index --embedded-nats --tsnet --tsnet-nats-port 4222
./dialtone.sh repl src_v1 join --nats-url nats://<server-host>:4222 --name <hostname> index
./dialtone.sh repl src_v1 service --mode install --repo timcash/dialtone --room index
./dialtone.sh repl src_v1 service --mode run --repo timcash/dialtone --check-interval 3m
./dialtone.sh repl src_v1 build
./dialtone.sh repl src_v1 release build v0.1.0
./dialtone.sh repl src_v1 release publish v0.1.0 timcash/dialtone
./dialtone.sh repl src_v1 test
```

## Interactive Commands
When running `run`, `serve`, or `join`, the REPL session supports internal management:
- `/ps`: List active subtones (background processes)
- `/kill <pid>`: Terminate a managed process
- `/repl src_v1 join <room-name>`: Leave current room and join another room
- `/<command>` or `/plugin src_vN command ...`: Send command to DIALTONE server
- `exit` / `quit`: Close the session

## NATS Model
- Global command subject: `repl.cmd`.
- Room event subjects: `repl.room.<room>`.
- Slash commands are published as `command` frames to `repl.cmd`.
- Non-slash text is published as `chat` frames to the current room.
- Server (`DIALTONE`) is the single command executor and publishes `line`/`server` output.
- Session lifecycle is tracked via `join` and `left` frames.

## Standalone Binary
```bash
./dialtone.sh repl src_v1 build
.dialtone/bin/repl-src_v1 serve --nats-url nats://0.0.0.0:4222 --room index --embedded-nats --tsnet --tsnet-nats-port 4222
.dialtone/bin/repl-src_v1 join --nats-url nats://<server-host>:4222 --name <hostname> index
.dialtone/bin/repl-src_v1 service --mode run --repo timcash/dialtone --nats-url nats://0.0.0.0:4222 --room index
```

### WSL + Embedded Tsnet
To avoid relying on Windows host networking, run the REPL server with embedded `tsnet`:

```bash
./dialtone.sh repl src_v1 serve --embedded-nats --nats-url nats://0.0.0.0:4222 --tsnet --tsnet-nats-port 4222
```

- This creates a dedicated tsnet node for the WSL server.
- The server logs a tailnet NATS endpoint (`nats://<tsnet-dns>:<port>`).
- Other hosts should join using that tsnet endpoint.
- In WSL, the tsnet hostname is auto-suffixed with `-wsl` unless explicitly set.

## Release Artifacts
`release build <version>` creates:
- `repl-src_v1-linux-amd64`
- `repl-src_v1-linux-arm64`
- `repl-src_v1-darwin-amd64`
- `repl-src_v1-darwin-arm64`
- `repl-src_v1-windows-amd64.exe`

`release publish` uploads those binaries to a GitHub release tag.

## Service & OS Persistence
The `service` command provides both a supervisor and automatic OS-level registration:
- `--mode install`: Registers the REPL supervisor as a **systemd user service** (Linux) or **launchd agent** (macOS).
- `--mode run`: Starts the supervisor in the foreground for log monitoring.
- `--mode status`: Checks the OS-level service status.

The supervisor maintains stability by:
- Polling GitHub Releases for newer architecture-matched binaries.
- Downloads new worker binaries to `~/.dialtone/repl/releases/`.
- Updates a `current` symlink and hot-swaps the worker via `SIGTERM`.
- Keeping the management layer alive even if the worker or network fails.

## Environment Discovery (`status`)
`repl src_v1 status` provides a comprehensive view of the local environment:
- **Networking**: NATS reachability and server presence probe.
- **VPN**: Tailscale (`tsnet`) configuration and authentication status.
- **Platform**: Chrome/Chromium installation path and version.

## Tests
`repl src_v1 test` runs:
- `src/plugins/repl/src_v1/test/cmd/main.go`
- `src/plugins/repl/src_v1/test/01_repl_core/suite.go`
- `src/plugins/repl/src_v1/test/02_proc_plugin/suite.go`
- `src/plugins/repl/src_v1/test/03_logs_plugin/suite.go`
- `src/plugins/repl/src_v1/test/04_test_plugin/suite.go`
- `src/plugins/repl/src_v1/test/05_chrome_plugin/suite.go`
- `src/plugins/repl/src_v1/test/06_go_bun_plugins/suite.go`
