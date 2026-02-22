# Bun Plugin

The Bun plugin runs managed Bun/Node tooling through `./dialtone.sh bun src_v1 ...`.

## Commands

```bash
./dialtone.sh bun src_v1 exec <bun-args...>
./dialtone.sh bun src_v1 run <script-and-args...>   # alias for exec run
./dialtone.sh bun src_v1 x <tool-and-args...>       # alias for exec x
./dialtone.sh bun src_v1 test
```

## Usage

### Run arbitrary Bun commands

```bash
./dialtone.sh bun src_v1 exec --version
./dialtone.sh bun src_v1 exec install
```

### Run project scripts

```bash
./dialtone.sh bun src_v1 run lint
./dialtone.sh bun src_v1 run build
```

### Run one-off tools

```bash
./dialtone.sh bun src_v1 x prettier --check .
```

### Run in a specific directory

Use `--cwd` with `exec`:

```bash
./dialtone.sh bun src_v1 exec --cwd src/plugins/dag/src_v2/ui run build
./dialtone.sh bun src_v1 exec --cwd src/plugins/dag/src_v2/ui install --force
```

## Testing

Run Bun plugin integration tests:

```bash
./dialtone.sh bun src_v1 test
```

Current test coverage verifies:
- stdout from Bun subprocesses is visible through `./dialtone.sh`
- stderr output and failure details propagate through `./dialtone.sh`

## Notes

- The plugin expects Bun at `DIALTONE_ENV/bun/bin/bun`.
- It prepends managed Bun and Node binaries to `PATH` for child commands.
- If Bun is missing, run `./dialtone.sh install`.
