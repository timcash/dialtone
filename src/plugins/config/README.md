# config plugin

`config` provides a shared runtime/env resolver for Dialtone plugins.

## Commands

```sh
./dialtone.sh config src_v1 help
./dialtone.sh config src_v1 runtime
./dialtone.sh config src_v1 apply
./dialtone.sh config src_v1 test
```

## Library

Import:

```go
import configv1 "dialtone/dev/plugins/config/src_v1/go"
```

Main helpers:
- `ResolveRuntime(start)` resolves repo root, src root, env file (`env/.env`), and managed Go/Bun paths.
- `LoadEnvFile(rt)` loads `env/.env` when present.
- `ApplyRuntimeEnv(rt)` exports `DIALTONE_*` vars and updates `PATH`.
- `NewPluginPreset(rt, plugin, version)` returns typed paths rooted at `PluginVersionRoot` (`src/plugins/<plugin>/<src_vN>`).
- `RepoPath(rt, ...)`, `SrcPath(rt, ...)`, `PluginPath(rt, plugin, version, ...)` remain available for generic cwd-independent paths.
