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
- `ResolveRuntime(start)` resolves repo root, src root, config file (`env/dialtone.json`), and managed Go/Bun paths.
- `LoadEnvFile(rt)` loads `env/dialtone.json` when present.
- `ApplyRuntimeEnv(rt)` exports `DIALTONE_*` vars and updates `PATH`.
- `NewPluginPreset(rt, plugin, version)` returns typed paths rooted at `PluginVersionRoot` (`src/plugins/<plugin>/<src_vN>`).
- `RepoPath(rt, ...)`, `SrcPath(rt, ...)`, `PluginPath(rt, plugin, version, ...)` remain available for generic cwd-independent paths.

## LLM Usage Guide

Use this plugin first whenever you write or update plugin code that needs file paths, env loading, or tool paths.

### Rules
- Do not hardcode paths like `filepath.Join(repoRoot, "src", "plugins", ...)`.
- Do not assume current working directory.
- Do not use `.env` files for Dialtone runtime configuration.
- Use `env/dialtone.json` as the single runtime config source.
- Resolve runtime once, then derive paths from presets.

### Standard pattern

```go
rt, err := configv1.ResolveRuntime("")
if err != nil {
  return err
}

preset := configv1.NewPluginPreset(rt, "robot", "src_v1")

uiDir := preset.UI
testDir := preset.Test
serverMain := preset.Join("cmd", "server", "main.go")
```

### Environment pattern

```go
rt, err := configv1.ResolveRuntime("")
if err != nil {
  return err
}
if err := configv1.LoadEnvFile(rt); err != nil {
  return err
}
if err := configv1.ApplyRuntimeEnv(rt); err != nil {
  return err
}
```

### What to use for paths
- Plugin source root (`src_vN`): `preset.PluginVersionRoot`
- UI source: `preset.UI`
- UI dist: `preset.UIDist`
- tests: `preset.Test`, `preset.TestCmd`
- command package: `preset.Cmd`
- go library package: `preset.Go`
- plugin shared bin dir: `preset.Bin`
- repo-level paths: `configv1.RepoPath(rt, ...)`
- src-level paths: `configv1.SrcPath(rt, ...)`

### Anti-patterns
- `os.Getwd()` + parent walking in each plugin.
- Repeated custom `findRepoRoot()` implementations.
- Inline joins for plugin/version directories in many files.

Centralize path setup once per plugin/src_vN (for example a local `ResolvePaths()` helper) and reuse it everywhere.
