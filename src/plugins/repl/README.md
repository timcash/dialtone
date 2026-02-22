# REPL Plugin

The REPL plugin provides:
- local interactive REPL (`run`)
- shared multi-client REPL over NATS (`serve`/`join`)
- update-aware service supervisor (`service`)
- release build/publish tooling for per-architecture binaries (`release`)

Prompts default to host identity (`<hostname>`) instead of `USER-1`.

## CLI
```bash
./dialtone.sh repl src_v1 help
./dialtone.sh repl src_v1 run
./dialtone.sh repl src_v1 status
./dialtone.sh repl src_v1 serve --nats-url nats://0.0.0.0:4222 --room main --embedded-nats
./dialtone.sh repl src_v1 join --nats-url nats://<server-ip>:4222 --room main --name <hostname>
./dialtone.sh repl src_v1 service --mode run --repo timcash/dialtone --nats-url nats://0.0.0.0:4222 --room main --check-interval 3m
./dialtone.sh repl src_v1 build
./dialtone.sh repl src_v1 release build v0.1.0
./dialtone.sh repl src_v1 release publish v0.1.0 timcash/dialtone
./dialtone.sh repl src_v1 test
```

## NATS Model
- REPL bus uses one subject per room: `repl.<room>`.
- User input is published as NATS `input` frames first.
- Server (`DIALTONE`) listens on that same subject and executes input frames.
- REPL stdout is replay from NATS `line`/`server` frames.
- Clients can detect an already-running server via probe/heartbeat frames on the same subject.

## Standalone Binary
```bash
./dialtone.sh repl src_v1 build
.dialtone/bin/repl-src_v1 serve --nats-url nats://0.0.0.0:4222 --room main --embedded-nats
.dialtone/bin/repl-src_v1 join --nats-url nats://<server-ip>:4222 --room main --name <hostname>
.dialtone/bin/repl-src_v1 service --mode run --repo timcash/dialtone --nats-url nats://0.0.0.0:4222 --room main
```

## Release Artifacts
`release build <version>` creates:
- `repl-src_v1-linux-amd64`
- `repl-src_v1-linux-arm64`
- `repl-src_v1-darwin-amd64`
- `repl-src_v1-darwin-arm64`
- `repl-src_v1-windows-amd64.exe`

`release publish` uploads those binaries to a GitHub release tag.

## Service Hot-Swap
`service --mode run` acts as a stable supervisor daemon:
- checks GitHub Releases every interval
- downloads newer architecture-matched worker binary
- stops old worker subprocess
- starts new worker subprocess
- keeps supervisor process alive during swap

## Tests
`repl src_v1 test` runs:
- `src/plugins/repl/src_v1/test/cmd/main.go`
- `src/plugins/repl/src_v1/test/01_repl_core/suite.go`
- `src/plugins/repl/src_v1/test/02_proc_plugin/suite.go`
- `src/plugins/repl/src_v1/test/03_logs_plugin/suite.go`
- `src/plugins/repl/src_v1/test/04_test_plugin/suite.go`
- `src/plugins/repl/src_v1/test/05_chrome_plugin/suite.go`
- `src/plugins/repl/src_v1/test/06_go_bun_plugins/suite.go`
