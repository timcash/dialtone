# REPL src_v3

> Important:
> - `./dialtone.sh --test` is deprecated. Use `./dialtone.sh repl src_v3 test ...`.
> - `--subtone` is internal-only and not a user command path.

```bash
# Core REPL src_v3 commands
./dialtone.sh repl src_v3 help
./dialtone.sh repl src_v3 install
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 lint
./dialtone.sh repl src_v3 check
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test

# Runtime commands
./dialtone.sh repl src_v3 run --nats-url nats://127.0.0.1:4222 --room index --name user
./dialtone.sh repl src_v3 leader --embedded-nats --nats-url nats://0.0.0.0:4222 --room index
./dialtone.sh repl src_v3 join --nats-url nats://127.0.0.1:4222 --room index --name observer
./dialtone.sh repl src_v3 status --nats-url nats://127.0.0.1:4222 --room index
./dialtone.sh repl src_v3 service --mode run --room index

# Injection + observability
./dialtone.sh repl src_v3 inject --user llm-codex go src_v1 version
./dialtone.sh repl src_v3 inject --user llm-codex --host grey go src_v1 version
./dialtone.sh repl src_v3 watch --subject 'repl.>' --filter 'DIALTONE:'
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid 12345 --lines 200

# Bootstrap + mesh helpers
./dialtone.sh repl src_v3 bootstrap
./dialtone.sh repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
./dialtone.sh repl src_v3 bootstrap-http --host 127.0.0.1 --port 8811

# Runtime cleanup
./dialtone.sh repl src_v3 process-clean --dry-run
./dialtone.sh repl src_v3 process-clean
./dialtone.sh repl src_v3 test-clean --dry-run
./dialtone.sh repl src_v3 test-clean

# Start REPL (default path)
./dialtone.sh

# Routed command behavior
./dialtone.sh go src_v1 version
./dialtone.sh go src_v1 version --host grey
./dialtone.sh go src_v1 version --host grey.shad-artichoke.ts.net:4222
./dialtone.sh go src_v1 version --target-host grey
./dialtone.sh go src_v1 version --ssh-host grey
./dialtone.sh test src_v1 test --user llm-codex
./dialtone.sh cloudflare src_v1 tunnel start demo --url http://127.0.0.1:8080 --token token-test

# Full REPL test run from tmp bootstrap workspace
./dialtone.sh repl src_v3 test

# Default test mode uses embedded local HTTP server:
#   curl http://shell.dialtone.earth:<local-port>/install.sh | bash -s -- repl src_v3 test
# (the test runner uses --resolve to map shell.dialtone.earth to localhost)

# Real external bootstrap mode (no local bootstrap server)
# Starts from empty /tmp folder and curls the real installer URL.
DIALTONE_REPL_V3_TEST_INSTALL_URL="https://shell.dialtone.earth/install.sh" \
./dialtone.sh repl src_v3 test

# Real integration test run:
# (requires valid env/dialtone.json keys + reachable wsl host)
DIALTONE_REPL_V3_TEST_INSTALL_URL="https://shell.dialtone.earth/install.sh" \
DIALTONE_REPL_V3_TEST_WSL_HOST="wsl.shad-artichoke.ts.net" \
DIALTONE_REPL_V3_TEST_WSL_USER="user" \
./dialtone.sh repl src_v3 test

```

## Target Architecture (Goal)

- Every host runs REPL src_v3 in the background (daemon mode).
- REPL leader with embedded NATS is the command/control plane for that host.
- `./dialtone.sh <command>` injects commands over NATS by default.
- `./dialtone.sh <command> --host <target>` routes the command to the target host REPL daemon by connecting directly to that host NATS endpoint (tsnet/tailnet path).
- SSH remains bootstrap/fallback for hosts that are not yet on REPL/NATS.
- `dialtone.sh` bootstrap flow should bring each host onto:
  - REPL daemon + embedded NATS
  - tailnet connectivity
  - Cloudflare `shell.dialtone.earth` bootstrap/load-balanced path
- End state: cross-host command execution is NATS-targeted REPL subtone execution, not SSH-first orchestration.
- Note: in this model, `--host` is transport routing for REPL/NATS targeting (legacy plugin-local `--host` behavior is intentionally deprecated).

## Current Behavior

- `./dialtone.sh` defaults to REPL v3.
- Regular commands are injected to REPL over NATS and executed as subtones.
- `--host <target>` and `--target-host <target>` on routed commands target remote REPL over NATS.
- `--host <mesh-node>` resolves mesh candidates and now retries transport (tailnet then LAN fallback).
- `--ssh-host <target>` is explicit SSH transport fallback (`ssh src_v1 run` path), separate from NATS routing.
- REPL output shows user command input and subtone lifecycle lines (`DIALTONE>` and `DIALTONE:<pid>`).
- Embedded NATS source of truth is the REPL leader process.
- REPL leader now attempts embedded tsnet by default; if native tailscale is already connected on the host, embedded tsnet startup is skipped automatically.
- Runtime config source of truth is `env/dialtone.json` (legacy `.env` and separate SSH host files are deprecated).
- `./dialtone.sh repl src_v3 test` bootstraps and tests from a tmp workspace.
- `./dialtone.sh repl src_v3 test` starts from an empty `/tmp` repo folder.
- Default `./dialtone.sh repl src_v3 test` mode starts an embedded local HTTP server in REPL src_v3 and executes `curl .../install.sh | bash`.
- Real external mode is enabled by `DIALTONE_REPL_V3_TEST_INSTALL_URL=https://shell.dialtone.earth/install.sh`.
- REPL v3 test steps are real integration steps by default (ssh->wsl, cloudflare tunnel start/stop, tsnet checks).
- `./dialtone.sh repl src_v3 test` starts with no `env/dialtone.json`; onboarding creates base runtime keys and injected REPL commands fill additional fields (for example mesh hosts).
- Use `subtone-list` to map pid -> command.
- Use `subtone-log --pid` to fetch exact subtone log file output.
- Use `watch` to stream REPL/NATS events directly for live debugging.

## Known Gaps

- Bootstrapping from `https://shell.dialtone.earth/install.sh` may still pull an older payload until shell serving/tunnel points at the latest gold workspace build.
- On older payloads, REPL leader tsnet startup can still fall back to interactive login URL instead of non-interactive auth-key provisioning.
- `--host` NATS routing is validated for LAN fallback today; tsnet ephemeral target routing depends on the shell payload including the latest tsnet/repl startup path.

## Cloudflare Tunnel Idea

- Goal: expose the local REPL/bootstrap HTTP entrypoint through Cloudflare so a remote machine can fetch `dialtone.sh` from a stable public URL.
- Current local command path is available now:
  - `./dialtone.sh cloudflare src_v1 tunnel start <name> --url http://127.0.0.1:<port> --token <token>`
- Practical use with REPL:
  - Start REPL service locally with `./dialtone.sh`
  - Start a tunnel pointing at the local REPL/bootstrap HTTP endpoint
  - Remote clients can `curl` the bootstrap script and then operate through REPL/NATS command injection.

## Watching Traffic

User-facing view:

```bash
./dialtone.sh repl src_v3 join --name observer --room index
```

Raw NATS bus view:

```bash
./dialtone.sh logs src_v1 stream --topic 'repl.>'
./dialtone.sh logs src_v1 stream --topic 'logs.test.>'
```

Independent passive tap repo:

- `https://github.com/timcash/dialtone-tap`
- reconnects automatically
- never starts NATS
- subscribe-only (non-interfering)
