# REPL v1

Minimal local REPL mod scaffold.

Run it through the repo CLI:

```bash
nix develop --command go run ./src/cli.go repl v1 run
nix develop --command go run ./src/cli.go repl v1 run --once "hello"
nix develop --command go run ./src/cli.go repl v1 logs --tail 20
```

Files stay local to this mod:

- runtime logs: `src/mods/repl/v1/runtime/repl.log`
- tests: `src/mods/repl/v1/*_test.go`
- Nix-backed operational commands: `src/mods/repl/v1/cli`
