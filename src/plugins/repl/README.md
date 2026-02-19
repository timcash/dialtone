# REPL Plugin

The REPL plugin provides focused tooling to develop and test the `dialtone.sh` interactive flow.

## Commands

```bash
./dialtone.sh repl install
./dialtone.sh repl test
./dialtone.sh repl help
```

After install, run the local Python bridge with pixi:

```bash
cd src/plugins/repl
pixi run bridge
```

## Purpose

- Validate `USER-1>` and `DIALTONE>` dialog behavior.
- Verify `@DIALTONE ...` request handling.
- Verify task-sign gating and subtone PID streaming output.

## Example

```bash
./dialtone.sh repl test --timeout 180
```