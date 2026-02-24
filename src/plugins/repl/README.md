# REPL Plugin

The REPL plugin provides:
- local interactive REPL (`run`)
- shared multi-client REPL over NATS (`leader`/`join`)
- update-aware service supervisor and OS persistence (`service`)
- release build/publish tooling for per-architecture binaries (`release`)
- environment discovery and bus health checks (`status`)

Prompts default to host identity (`<hostname>`) instead of `USER-1`.

## CLI
```bash
./dialtone.sh repl src_v1 help
./dialtone.sh repl src_v1 run
./dialtone.sh repl src_v1 status
./dialtone.sh repl src_v1 leader --nats-url nats://0.0.0.0:4222 --room index --embedded-nats --tsnet --tsnet-nats-port 4222
./dialtone.sh repl src_v1 join --nats-url nats://<server-host>:4222 --name <hostname> index
./dialtone.sh repl src_v1 service --mode install --repo timcash/dialtone --room index
./dialtone.sh repl src_v1 service --mode run --repo timcash/dialtone --check-interval 3m
./dialtone.sh repl src_v1 build
./dialtone.sh repl src_v1 deploy --host <robot-host> --user <robot-user> --pass <robot-pass> --service --embedded-nats=false
./dialtone.sh repl src_v1 release build v0.1.0
./dialtone.sh repl src_v1 release publish v0.1.0 timcash/dialtone
./dialtone.sh repl src_v1 test
```

## Interactive Commands
When running `run`, `leader`, or `join`, the REPL session supports internal management:
- `/ps`: List active subtones (background processes)
- `/kill <pid>`: Terminate a managed process
- `/repl src_v1 join <room-name>`: Leave current room and join another room
- `/<command>` or `/plugin src_vN command ...`: Send command to DIALTONE leader
- `exit` / `quit`: Close the session

## NATS Model
- One host runs `leader` and acts as the REPL **leader**.
- The leader is the only process that executes commands (`DIALTONE>` command handler).
- Clients run `join` and publish all input to NATS first.
- Global command subject: `repl.cmd`.
- Room event subjects: `repl.room.<room>`.
- Slash commands are published as `command` frames to `repl.cmd`.
- Non-slash text is published as `chat` frames to the current room.
- Leader (`DIALTONE`) is the single command executor and publishes `line`/`server` output.
- Session lifecycle is tracked via `join` and `left` frames.

### Leader Offline Behavior
- If the leader process stops but NATS stays online:
- Chat events can still be published to room subjects.
- Slash commands (`/...`) are not executed because no leader is subscribed to `repl.cmd`.
- `status` probes will report no active server heartbeat for that room.
- If the shared NATS broker goes offline:
- Clients cannot publish/subscribe; room updates and commands both stop.
- Existing `join` sessions disconnect from transport and must reconnect once NATS is reachable.
- Recovery:
- Start a leader again with `leader` against the shared NATS URL.
- Clients re-`join` (or reconnect) and command execution resumes.
- There is no NATS clustering or leaf-node topology in the current implementation.

## Standalone Binary
```bash
./dialtone.sh repl src_v1 build
.dialtone/bin/repl-src_v1 leader --nats-url nats://0.0.0.0:4222 --room index --embedded-nats --tsnet --tsnet-nats-port 4222
.dialtone/bin/repl-src_v1 join --nats-url nats://<server-host>:4222 --name <hostname> index
.dialtone/bin/repl-src_v1 service --mode run --repo timcash/dialtone --nats-url nats://0.0.0.0:4222 --room index
```

### WSL + Embedded Tsnet
To avoid relying on Windows host networking, run the REPL leader with embedded `tsnet`:

```bash
./dialtone.sh repl src_v1 leader --embedded-nats --nats-url nats://0.0.0.0:4222 --tsnet --tsnet-nats-port 4222
```

- This creates a dedicated tsnet node for the WSL leader.
- The leader logs a tailnet NATS endpoint (`nats://<tsnet-dns>:<port>`).
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

## Remote Deploy (Robot)
`deploy` pushes a platform-matched `repl-src_v1` binary to a remote host over SSH.

- Defaults to `ROBOT_HOST`, `ROBOT_USER`, and `ROBOT_PASSWORD` from `env/.env`.
- `--service` installs/restarts `dialtone-repl.service` on the remote host.
- Remote service runs `repl-src_v1 service --mode run`, so updates are pulled automatically from GitHub Releases.

## Environment Discovery (`status`)
`repl src_v1 status` provides a comprehensive view of the local environment:
- **Networking**: NATS reachability and leader presence probe.
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
