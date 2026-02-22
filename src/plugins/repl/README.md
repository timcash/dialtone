# REPL Plugin

The REPL plugin tests interactive `dialtone.sh` behavior (`USER-1>` / `DIALTONE>` flow).

## CLI
```bash
./dialtone.sh repl src_v1 help
./dialtone.sh repl src_v1 test
```

`repl src_v1 test` runs:
- `src/plugins/repl/src_v1/test/cmd/main.go`
- `src/plugins/repl/src_v1/test/01_repl_core/suite.go`
- `src/plugins/repl/src_v1/test/02_proc_plugin/suite.go`
- `src/plugins/repl/src_v1/test/03_logs_plugin/suite.go`
- `src/plugins/repl/src_v1/test/04_test_plugin/suite.go`
- `src/plugins/repl/src_v1/test/05_chrome_plugin/suite.go`
- `src/plugins/repl/src_v1/test/06_go_bun_plugins/suite.go`

Current suite coverage:
- `01_repl_core`
  - verifies REPL-only behavior: help text correctness, input handling, `USER-1>`/`DIALTONE>` line formatting
- `02_proc_plugin`
  - runs `proc src_v1 test` through REPL subtone flow
- `03_logs_plugin`
  - runs `logs src_v1 test` through REPL subtone flow
- `04_test_plugin`
  - runs `test src_v1 test` through REPL subtone flow
- `05_chrome_plugin`
  - runs `chrome src_v1 list` through REPL subtone flow
- `06_go_bun_plugins`
  - runs `go src_v1 test` then `bun src_v1 test` through REPL subtone flow

## Notes
- REPL tests are implemented with shared `test` plugin (`testv1.StepContext`) and `logs` plugin (`logs/src_v1/go`) patterns.
- Suites are arranged bottom-up and execute foundational plugin checks via REPL-managed subtones.
- REPL commands are entered directly (for example `logs src_v1 test`); no `@DIALTONE` prefix is used.
