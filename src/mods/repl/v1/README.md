# REPL v1

Minimal local REPL mod scaffold.

Run it through the repo CLI:

```bash
./dialtone_mod repl v1 run
./dialtone_mod repl v1 run --once "hello"
./dialtone_mod repl v1 logs --tail 20
```

Or enter the focused flake shell first:

```bash
nix develop .#repl-v1
./dialtone_mod repl v1 test
```

Files stay local to this mod:

- runtime logs: `src/mods/repl/v1/runtime/repl.log`
- tests: `src/mods/repl/v1/*_test.go`
- Nix-backed operational commands: `src/mods/repl/v1/cli`
