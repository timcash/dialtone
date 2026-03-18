# CAD Plugin

`cad` is a versioned plugin. Use `src_v1` commands through `./dialtone.sh`.

## Commands

```bash
# Start the CAD backend server on port 8081.
./dialtone.sh cad src_v1 serve

# Legacy alias still works, but src_v1 is the preferred form.
./dialtone.sh cad server

# Run the CAD smoke tests.
./dialtone.sh cad src_v1 test

# Show command help.
./dialtone.sh cad src_v1 help
```

## REPL Runtime

Plain `./dialtone.sh cad src_v1 ...` commands route through `repl src_v3` by default.

Expected `DIALTONE>` summaries:

```text
DIALTONE> cad serve: starting backend on 127.0.0.1:8081
DIALTONE> cad test: running 2 suite steps
DIALTONE> cad test: suite passed
```

Detailed CAD server output and test logs stay in the subtone log. Use:

```bash
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```
