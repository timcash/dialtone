# Mods CLI Architecture Plan

## Intended Reader

This file is for the next engineer or LLM agent who needs to continue the mods transition without rediscovering the architecture from scratch.

Read order:

1. `src/mods/README.md`
2. this file
3. the target mod README you are changing
4. `src/mods.go`
5. `src/internal/modstate/modstate.go`

Use this file as the transition plan. Use `src/mods/README.md` as the stable system guide.

## Purpose

This plan defines a clean architecture for the mods system so that:

- every mod has one small Go CLI wrapper at `src/mods/<mod>/<version>/cli/main.go`
- `src/mods.go` dispatches through those CLI wrappers instead of mixing CLI and version-root entrypoints
- `dialtone_mod` remains the stable user-facing shell command
- the standalone `dialtone` background process remains the process manager and queue owner
- routed plain `./dialtone_mod ...` commands still run visibly in `dialtone-view`
- build outputs land under `bin/mods/<mod>/<version>/` so the `bin` tree mirrors `src/mods`

This is a migration plan, not a flag day rewrite.

## Decisions Already Made

These decisions are already settled and should not be reopened during normal transition work:

- `dialtone_mod` remains the stable user-facing shell command
- `dialtone` remains the standalone background process manager
- SQLite remains the source of truth for queue, worker, status, protocol, and test state
- routed plain `./dialtone_mod ...` commands should execute in `dialtone-view`
- `codex-view` is for prompts and reasoning, not for executing routed worker commands
- every real mod version should expose a small Go CLI wrapper at `src/mods/<mod>/<version>/cli/main.go`
- every mod CLI should recognize `install`, `build`, `format`, and `test`
- build outputs should mirror the source tree under `bin/mods/<mod>/<version>/`

## Transition Guardrails

While migrating the system:

- do not change the user-facing command shape from plain `./dialtone_mod ...`
- do not move queue ownership out of `dialtone`
- do not make `src/mods.go` CLI-only until all real mods have working CLI wrappers
- do not remove a version-root runtime entrypoint until its commands have been moved behind the CLI wrapper
- do not let routed commands execute in the caller terminal or in `codex-view`
- do not reintroduce flat build outputs like `bin/mod-v1` or `bin/repl-v1`
- do not bypass the Nix-backed mod workflow with host `go build`, `gofmt`, or `go test`
- do not break `./dialtone_mod shell v1 test-basic` or `./dialtone_mod test v1 start`

## New Agent Start Checklist

Before making code changes:

1. read `src/mods/README.md` and this file completely
2. confirm the shell/control-plane baseline still works:
   - `./dialtone_mod shell v1 test-basic`
   - `./dialtone_mod test v1 start`
3. scan mod CLI coverage and pick one bounded migration slice
4. add or normalize tests before changing behavior
5. keep the visible shell workflow centered on `shell v1`
6. update the target mod README and this plan if the transition state changes materially

Recommended first commands for a new agent:

```sh
# inspect mod versions
find src/mods -mindepth 2 -maxdepth 2 -type d | sort

# inspect CLI coverage
for d in $(find src/mods -mindepth 2 -maxdepth 2 -type d | sort); do
  if [ -f "$d/cli/main.go" ]; then
    printf 'HAS_CLI\t%s\n' "$d"
  else
    printf 'NO_CLI\t%s\n' "$d"
  fi
done

# baseline control plane
./dialtone_mod shell v1 test-basic
./dialtone_mod test v1 start
```

## Target Architecture

### Layer 0: `dialtone_mod` shell wrapper

`dialtone_mod` should stay small and stable.

Responsibilities:

- find the repo root and bootstrap state directory
- ensure the standalone `dialtone` binary exists
- ensure the `dialtone` daemon is running
- send routed commands to `dialtone v1 queue`
- run non-routed commands through the Go dispatcher
- provide a small amount of bootstrap/runtime environment setup

Non-goals:

- no per-mod business logic
- no duplicate queue implementation
- no duplicate status renderer for mod-specific behavior
- no separate command parser for every mod

### Layer 1: `dialtone` daemon

The standalone `dialtone` binary is the control plane and process manager.

Responsibilities:

- own SQLite queue/state
- supervise the long-lived shell worker in `dialtone-view`
- record command lifecycle, health, heartbeats, and outputs
- provide fast status/state reporting
- queue plain `./dialtone_mod ...` commands without changing the user-facing command shape

Important rule:

- `dialtone` should record execution metadata in SQLite, but it should not invent a second user command language

Recommended metadata to keep on queued rows:

- original argv / command text
- actor/source such as `user`, `codex`, `test-harness`
- route mode such as `routed` or `direct`
- prompt/protocol linkage when applicable
- target pane / worker identity

That gives the control plane enough information to run the command correctly without mutating the command surface.

### Layer 2: `src/mods.go`

`src/mods.go` should become a thin router.

Responsibilities:

- normalize `<mod> <version> <command>`
- map aliases like `mods -> mod`
- decide routed vs direct execution
- resolve the mod entrypoint
- launch the mod CLI wrapper

Hard rule:

- after the migration, `src/mods.go` should resolve through `cli/main.go` for every real mod version

Non-goals:

- no direct dependency on version-root `main.go` for normal mod execution
- no mod-specific branching beyond routing and command-name aliases

### Layer 3: per-mod CLI wrapper

Every mod version should expose `src/mods/<mod>/<version>/cli/main.go`.

The CLI wrapper is the mod contract with `src/mods.go`.

Required commands for every mod:

- `install`
- `build`
- `format`
- `test`

Allowed extra commands:

- mod-specific runtime and admin commands such as `start`, `status`, `queue`, `service`, `tab`, `hosts`, `run`, or `help`

Rule:

- the exact behavior of `install|build|format|test` can differ by mod
- the command names themselves are the shared contract

Implementation rule:

- `cli/main.go` should stay small and mostly parse/dispatch
- heavy logic should live in helper files or a package under that mod

### Layer 4: runtime binaries and helper packages

Some mods need a real standalone runtime binary.

Examples:

- `dialtone/v1` daemon
- `db/v1` native/zig SQLite extension binary

For those mods:

- the CLI wrapper should own `install|build|format|test`
- the runtime binary may still have its own `main.go`
- `src/mods.go` should still target the CLI wrapper, not the runtime `main.go`

This keeps the user-facing mod contract consistent while preserving standalone binaries where needed.

## Bin Layout

All mod builds should write into:

```text
bin/mods/<mod>/<version>/
```

Recommended primary artifact names:

- `bin/mods/mod/v1/mod`
- `bin/mods/repl/v1/repl`
- `bin/mods/dialtone/v1/dialtone`
- `bin/mods/db/v1/dialtone_db`

Rules:

- one mod may emit more than one artifact if needed
- the mod chooses the artifact name
- the directory structure must mirror `src/mods`
- `build` should never write to a flat repo-root `bin/<name>` path once the migration is complete

## Current Gaps

### Missing `cli/main.go`

These mod versions currently have no CLI wrapper:

- `src/mods/db/v1`
- `src/mods/dialtone/v1`
- `src/mods/mesh/v3`
- `src/mods/ssh/v1`

### CLIs missing some required contract commands

This quick scan looked only for `install|build|format|test` in `cli/main.go`.

- `chrome/v1`: complete
- `mod/v1`: complete
- `repl/v1`: complete
- `codex/v1`: missing `install`, `build`, `format`, `test`
- `ghostty/v1`: missing `install`, `build`, `format`, `test`
- `mosh/v1`: only `install` present
- `shell/v1`: only `test` present
- `test/v1`: missing `install`, `build`, `format`, `test`
- `tmux/v1`: missing `install`, `build`, `format`, `test`
- `tsnet/v1`: only `install` present

### Dispatcher inconsistency

Today both of these still special-case the CLI only for `install|build|format|test`:

- `src/mods.go`
- `src/internal/modstate/modstate.go`

That is the main architecture mismatch. It means many commands still run via version-root `main.go`.

### Build output inconsistency

Some existing build helpers still target flat paths under `bin/`, for example:

- `src/mods/mod/v1/cli/build.go`
- `src/mods/repl/v1/cli/build.go`
- `src/mods/chrome/v1/cli/build.go`

Those need to move to `bin/mods/<mod>/<version>/`.

### Shared helper gap

There is not yet a single shared helper package for the common mod-CLI contract.

That means many wrappers still duplicate logic for:

- repo-root discovery
- output-path construction
- `gofmt` / `go test` / `go build` execution
- Nix-aware command launching
- usage and exit handling

This should be consolidated before or while the broader CLI migration proceeds.

## File Map For The Transition

These files are the main control points for the migration:

- `dialtone_mod`
  Stable user-facing shell wrapper and daemon bootstrap path
- `src/mods.go`
  Main Go dispatcher that should become CLI-only for real mods
- `src/internal/modstate/modstate.go`
  SQLite-backed mod registry and entrypoint resolution
- `src/mods/<mod>/<version>/cli/main.go`
  Per-mod contract entrypoint
- `src/mods/<mod>/<version>/main.go`
  Legacy runtime entrypoint or standalone binary entrypoint when a real runtime binary is needed
- `src/mods/README.md`
  Stable system guide for how the future system should work

## Suggested Slice Order

Do the transition in small vertical slices instead of one large rewrite.

Suggested order:

1. add `src/internal/modcli`
2. migrate one missing-CLI mod with strong architectural value:
   - `dialtone/v1`
3. migrate the next missing-CLI runtime mod:
   - `db/v1`
4. migrate the remaining missing-CLI mods:
   - `ssh/v1`
   - `mesh/v3`
5. fill in missing `install|build|format|test` for existing CLI mods
6. standardize build paths under `bin/mods/...`
7. flip dispatcher resolution to CLI-only

Why this order:

- `dialtone/v1` and `db/v1` set the pattern for standalone runtime binaries
- once that pattern is clear, the simpler CLI gaps become mechanical
- only after every mod can be reached through a wrapper should the dispatcher stop using version-root fallbacks

## Clean Migration Strategy

### Phase 1: define the shared CLI scaffold

Add one small shared helper package, for example:

- `src/internal/modcli`

It should centralize:

- repo-root discovery
- standard bin path generation
- standard `go build` output path generation
- common `format` and `test` helpers
- Nix-aware execution helpers
- common usage/error helpers

Goal:

- remove repeated path and shell logic from every CLI wrapper

Suggested contents for `src/internal/modcli`:

- `FindRepoRoot()`
- `ModDir(mod, version)`
- `CLIDir(mod, version)`
- `BinDir(mod, version)`
- `EnsureBinDir(mod, version)`
- `GoBuildOutput(mod, version, artifact)`
- `RunNixGo(...)`
- `RunFormat(...)`
- `RunTest(...)`
- common help/exit helpers where that actually reduces duplication

### Phase 2: make every mod expose the contract

For every real mod version:

- add `cli/main.go` if missing
- add `install`, `build`, `format`, `test`
- keep or add any mod-specific runtime commands in that CLI

Important detail:

- do not remove runtime functionality while adding the contract
- each CLI should support both the required basic commands and the existing runtime/admin surface

Examples:

- `codex/v1` should keep `start|status` and add `install|build|format|test`
- `shell/v1` should keep `start|run|serve|status|test...` and add `install|build|format`
- `dialtone/v1` should add a CLI wrapper even though it also has a standalone daemon binary

### Phase 3: separate CLI parsing from runtime logic

As each mod is touched:

- move heavy runtime logic out of `cli/main.go` into helper files or packages
- stop using version-root `main.go` as the normal dispatcher target

Recommended pattern:

- `cli/main.go`: parse top-level commands
- `cli/*.go`: command handlers and build/install/test helpers
- optional `internal/` or package files: reusable runtime logic
- optional version-root `main.go`: only for true standalone binaries that are launched directly outside the dispatcher

### Phase 4: switch the dispatcher to CLI-only resolution

Once every mod has a working CLI wrapper:

1. change `src/mods.go` to always resolve `src/mods/<mod>/<version>/cli`
2. change `src/internal/modstate/modstate.go` to do the same
3. remove the `shouldUseModCLI` / `shouldUseCLICommand` split
4. treat missing CLI wrappers as a registry error

This is the actual architecture simplification.

### Phase 5: align `dialtone_mod` with the new contract

Keep `dialtone_mod` as the stable shell surface.

Its runtime behavior should become:

1. parse high-level bootstrap exceptions
2. ensure the standalone `dialtone` process manager is healthy
3. if the command is routed, send the original argv to `dialtone v1 queue`
4. if the command is direct, invoke the Go dispatcher

Important detail:

- the worker in `dialtone-view` should continue to execute the same plain `./dialtone_mod ...` command text the user or Codex typed

That keeps the visible workflow stable even while the internals migrate to CLI-only entrypoints.

One important constraint:

- `dialtone_mod` should still be able to bootstrap and call into `dialtone` even before the full CLI migration is complete
- the CLI migration should not require a flag day change to the shell wrapper

### Phase 6: keep the daemon boundary clean

The `dialtone` daemon should remain outside Nix if needed, but:

- `install|build|format|test` for `dialtone/v1` should still run through its CLI wrapper
- the daemon build should still happen inside the expected Nix-backed development flow

Recommended split for `dialtone/v1`:

- `cli/main.go`: mod contract plus daemon admin commands
- daemon runtime package or version-root main: actual long-lived server
- `build`: emit `bin/mods/dialtone/v1/dialtone`
- `dialtone_mod`: ensure/exec that built binary

## Testing Plan

Each migration slice should prove three things:

- the target mod CLI contract works
- dispatcher behavior did not regress
- the visible control plane still works

Minimum validation for a slice that changes mod dispatch or the control plane:

```sh
./dialtone_mod shell v1 test-basic
./dialtone_mod shell v1 run --wait-seconds 120 \
  "clear && cd /Users/user/dialtone/src && go test ./mods/<mod-name>/<version>/..."
./dialtone_mod test v1 start
```

### Contract tests

Add one shared test pattern that verifies per mod:

- CLI exists
- `help` renders
- `install|build|format|test` are recognized
- `build` targets `bin/mods/<mod>/<version>/...`

### Dispatcher tests

Add tests for:

- `src/mods.go` always resolving CLI entrypoints
- routing behavior staying unchanged
- alias handling such as `mods -> mod`
- correct error when a mod has no CLI wrapper

### Daemon and worker tests

Keep `test v1 start` as the visible end-to-end proof.

After the CLI migration, it should still prove:

- Codex runs a plain `./dialtone_mod ...` command from `codex-view`
- the command is queued into SQLite
- the worker in `dialtone-view` executes it
- success, failure, long-running, and background cases remain visible and durable

## Recommended Execution Order

1. Add `src/internal/modcli` shared helpers.
2. Add missing CLIs for `db/v1`, `dialtone/v1`, `mesh/v3`, and `ssh/v1`.
3. Fill in missing `install|build|format|test` commands for existing CLIs.
4. Standardize all build outputs under `bin/mods/<mod>/<version>/`.
5. Move any remaining runtime parsing out of version-root dispatcher paths into the CLIs.
6. Switch `src/mods.go` and `modstate.ResolveEntrypoint` to CLI-only resolution.
7. Add contract tests and rerun `./dialtone_mod test v1 start`.

## Concrete Success Criteria

The migration is done when all of the following are true:

- every real mod version has `cli/main.go`
- every CLI recognizes `install|build|format|test`
- `src/mods.go` never dispatches to version-root `main.go` for normal mod execution
- `dialtone_mod` still works as the user-facing command
- routed commands still queue into SQLite and run in `dialtone-view`
- `dialtone` still acts as the background process manager
- built artifacts land under `bin/mods/<mod>/<version>/`
- `test v1 start` still proves `codex-view` and `dialtone-view` cooperate correctly

## What “Done” Looks Like For One Mod

A single mod transition is complete when:

- `src/mods/<mod>/<version>/cli/main.go` exists
- that CLI recognizes `install`, `build`, `format`, and `test`
- mod-specific runtime/admin commands are reachable through the CLI too
- `build` writes to `bin/mods/<mod>/<version>/`
- focused recursive tests for that mod pass
- `src/mods.go` no longer needs the version-root entrypoint for that mod’s normal command surface
- the mod README documents the current accepted behavior
