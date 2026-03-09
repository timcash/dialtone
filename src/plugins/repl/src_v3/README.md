# REPL src_v3

```bash
# REPL src_v3 command reference
./dialtone.sh repl src_v3 help
./dialtone.sh repl src_v3 install
./dialtone.sh repl src_v3 format
./dialtone.sh repl src_v3 lint
./dialtone.sh repl src_v3 check
./dialtone.sh repl src_v3 build
./dialtone.sh repl src_v3 run
./dialtone.sh repl src_v3 leader --embedded-nats --nats-url nats://0.0.0.0:4222 --room index
./dialtone.sh repl src_v3 join --nats-url nats://127.0.0.1:4222 --room index --name observer
./dialtone.sh repl src_v3 inject --user llm-codex go src_v1 version
./dialtone.sh repl src_v3 bootstrap
./dialtone.sh repl src_v3 bootstrap --apply --wsl-host wsl.shad-artichoke.ts.net --wsl-user user
./dialtone.sh repl src_v3 add-host --name wsl --host wsl.shad-artichoke.ts.net --user user
./dialtone.sh repl src_v3 status
./dialtone.sh repl src_v3 service --mode run --room index
./dialtone.sh repl src_v3 test

# Through dialtone default routing (injected via REPL)
./dialtone.sh go src_v1 version
./dialtone.sh test src_v1 test --user llm-codex

# Full bootstrap-style test (tmp workspace + local tarball webserver emulating GitHub download)
./dialtone.sh --test

# In-repo-only REPL test mode
DIALTONE_REPL_V3_TEST_MODE=inside ./dialtone.sh --test
```

## Model

- `./dialtone.sh` defaults to REPL v3.
- Regular commands are injected to REPL over NATS and executed as subtones.
- REPL output shows user command input and subtone lifecycle lines (`DIALTONE>` and `DIALTONE:<pid>`).
- Embedded NATS source of truth is the REPL leader process.

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

- `https://github.com/timcash/repl-nats-tap`
- reconnects automatically
- never starts NATS
- subscribe-only (non-interfering)
