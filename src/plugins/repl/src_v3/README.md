# REPL src_v3

```bash
# Cleanup runtime/test artifacts
./dialtone.sh repl src_v3 process-clean --dry-run
./dialtone.sh repl src_v3 process-clean
./dialtone.sh repl src_v3 test-clean --dry-run
./dialtone.sh repl src_v3 test-clean

# REPL src_v3 developer commands
./dialtone.sh repl src_v3 help
./dialtone.sh repl src_v3 install
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 lint
./dialtone.sh repl src_v3 check
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 test
./dialtone.sh repl src_v3 status
./dialtone.sh repl src_v3 service --mode run --room index

# Start REPL (default path)
./dialtone.sh

# Explicit leader/join commands
./dialtone.sh repl src_v3 leader --embedded-nats --nats-url nats://0.0.0.0:4222 --room index
./dialtone.sh repl src_v3 join --nats-url nats://127.0.0.1:4222 --room index --name observer

# Inject commands via NATS into REPL
./dialtone.sh repl src_v3 inject --user llm-codex go src_v1 version
./dialtone.sh repl src_v3 inject --user llm-codex help
./dialtone.sh repl src_v3 inject --user llm-codex ps

# Default routed commands (also injected through REPL)
./dialtone.sh go src_v1 version
./dialtone.sh test src_v1 test --user llm-codex
./dialtone.sh cloudflare src_v1 tunnel start demo --url http://127.0.0.1:8080 --token token-test

# Bootstrap helpers
./dialtone.sh repl src_v3 bootstrap
./dialtone.sh repl src_v3 bootstrap --apply --wsl-host wsl.shad-artichoke.ts.net --wsl-user user
./dialtone.sh repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user

# Full REPL test run from tmp bootstrap workspace
./dialtone.sh --test

# Optional in-repo test mode
DIALTONE_REPL_V3_TEST_MODE=inside ./dialtone.sh --test
```

## Current Behavior

- `./dialtone.sh` defaults to REPL v3.
- Regular commands are injected to REPL over NATS and executed as subtones.
- REPL output shows user command input and subtone lifecycle lines (`DIALTONE>` and `DIALTONE:<pid>`).
- Embedded NATS source of truth is the REPL leader process.
- `./dialtone.sh --test` bootstraps and tests from a tmp workspace.

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
