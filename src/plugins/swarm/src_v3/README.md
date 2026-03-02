# Swarm Plugin (`src_v3`)

`src_v3` is a minimal swarm/transport plugin built around a native C binary that uses `libudx` for reliable UDP streams. It includes:
- A CLI entrypoint (`./dialtone.sh swarm src_v3 ...`)
- A native transport binary (`dialtone_swarm_v3.c`)
- A lightweight rendezvous relay web server (`relay_web/main.go`)
- Local and rendezvous self-tests (`test/`)

## What It Does

The core binary (`dialtone_swarm_v3_*`) opens a UDX socket, connects to a peer, and can send/receive messages over UDP using stream IDs.

Main runtime behaviors:
- Binds UDP on `--bind-ip/--bind-port`
- Connects to peer on `--peer-ip/--peer-port`
- Supports sender or receiver mode (`--no-send`)
- Periodic send loop (`--count`, `--interval-ms`)
- Optional timed shutdown (`--exit-after-ms`)

## How `libudx` Is Used

This plugin vendors `libudx` as a git submodule at:
- `src/plugins/swarm/src_v3/libudx`

Build flow in `go/swarm.go`:
1. Ensure submodule exists (`git submodule update --init --recursive ...`)
2. Build `libudx` and bundled `libuv` static libs:
   - `npm install`
   - `bare-make generate`
   - `bare-make build`
3. Compile `dialtone_swarm_v3.c` and link statically against:
   - `libudx.a`
   - `libuv.a`

UDX calls used in C:
- `udx_init`
- `udx_socket_init`, `udx_socket_bind`
- `udx_stream_init`, `udx_stream_connect`
- `udx_stream_read_start`, `udx_stream_write`

## Commands

Run via:
```bash
./dialtone.sh swarm src_v3 <command> [args]
```

Supported commands:
- `install`  
  Installs system deps (unless `--skip-apt`), global `bare-make`, and `libudx` npm deps.
- `build [--arch host|x86_64|arm64|all]`  
  Builds static binaries:
  - `dialtone_swarm_v3_x86_64`
  - `dialtone_swarm_v3_arm64`
- `test [--mode local|rendezvous|all] [--rendezvous-url URL]`  
  Runs Go test harness in `test/cmd/main.go`.
- `relay serve [--listen :8080]`  
  Runs relay tracker/web UI (`relay_web/main.go`).
- `deploy --host <name|csv|all|ip> [--user U] [--pass P] [--service=true|false]`  
  Builds for remote arch, uploads binary via SSH, and starts a long-running swarm service by default.  
  Example: `./dialtone.sh swarm src_v3 deploy --host all`
- `verify-host-builds ...`  
  SSHes into configured hosts and verifies native host builds.

## Relay Server

`relay_web/main.go` provides:
- `GET /health`
- `POST /api/register` (topic + who + UDP port, returns peers)
- `POST /api/ping`
- `GET /api/peers?topic=...`
- Static UI from `relay_web/static`

Relay listen address defaults to `:8080` and can be changed with:
- CLI: `relay serve --listen 0.0.0.0:18080`
- Env: `RELAY_LISTEN`

## Testing Notes

Test entrypoint:
- `src/plugins/swarm/src_v3/test/cmd/main.go`

Modes:
- `local`: loopback sender/receiver self-test
- `rendezvous`: peer discovery through relay + send/receive validation

Current default rendezvous URL in tests:
- `https://relay.dialtone.earth`

Implementation detail to be aware of:
- `RunRendezvousSelfTest` currently logs discovered peer IPs, then forces both peers to `127.0.0.1` for transport execution. So this self-test validates rendezvous API behavior plus local transport, not full cross-host LAN traversal.
