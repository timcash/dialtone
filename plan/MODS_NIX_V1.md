# Mods Nix v1 Plan

## Codex Workflow Note

For repeated work on one mod, the preferred Codex workflow is a persistent `nix develop` shell, not one fresh shell per command.

In practice:

- start one PTY session with `nix develop .#repl-v1` or `nix develop .#ssh-v1`
- keep that shell alive
- send later commands into that same shell session

That gives two benefits:

1. all later commands in that session use the Nix-provided tools from the flake shell
2. repeated work is faster because Codex does not pay the `nix develop` startup cost on every command

`./dialtone_mod` is still useful inside that shell because it keeps mod routing consistent, but the fastest development loop is:

```bash
nix develop .#repl-v1
./dialtone_mod repl v1 test
go test ./src/mods/repl/v1/...
gofmt -w ./src/mods/repl/v1
```

The same rule applies to `ssh-v1`: enter the flake shell once, then run repeated `./dialtone_mod ssh v1 ...` commands from inside it.

## Current State

Dialtone now has a partially migrated flake-first mod workflow.

Implemented:

- the root flake defines dedicated shells for `repl-v1` and `ssh-v1`
- `./dialtone_mod repl v1 ...` enters `nix develop path:$REPO_ROOT#repl-v1` when needed
- `./dialtone_mod ssh v1 ...` enters `nix develop path:$REPO_ROOT#ssh-v1` when needed
- the flake shells export `DIALTONE_GO_BIN` and `DIALTONE_SSH_BIN`
- `repl v1` uses the shell-provided `go`
- `ssh v1` requires the shell-provided Nix `ssh` binary and no longer supports the old nested `nix shell -f ... openssh` path

Still split / not finished:

- mods other than `repl v1` and `ssh v1` still use the older ad hoc package-shell path through `./dialtone_mod`
- some mod CLIs still construct their own Nix bootstrap logic
- the flake is not yet the single command surface for all active mods
- repo docs still assume a mix of workflows in places

That split causes three concrete problems:

1. Reproducibility is weaker than it should be.
   The repo lock file is authoritative for `repl v1` and `ssh v1`, but not yet for the whole mod surface.

2. Startup is noisier and slower than it should be.
   Outside a persistent shell, `./dialtone_mod` still has to enter `nix develop` per command. That is correct, but slower than working inside one long-lived shell session.

3. Mod logic is more duplicated than it should be.
   The first two mods are centralized, but other mods still carry their own bootstrap logic.

## Goal

Make the repository flake the single source of truth for mod development environments and `dialtone_mod` execution.

The desired developer experience is:

- enter the repo once with `nix develop` or `direnv`
- run `./dialtone_mod <mod> <version> <command>` without re-solving ad hoc package sets
- optionally enter a focused shell for one mod
- have a pinned, reproducible environment across machines
- keep mod-specific runtime/setup logic in the mod, but keep Nix environment construction in one place

## Non-Negotiable Principles

1. The root flake owns the toolchain.
   `go`, `git`, `bash`, `tmux`, `bun`, `openssh`, and other shared tooling should come from the root flake, not from host tools or scattered `nix shell nixpkgs#...` calls.

2. `dialtone_mod` should be a thin wrapper.
   It should route commands and enter the appropriate flake-backed environment, not dynamically invent package resolution policy.

3. Mod manifests should remain simple.
   `src/mods/<mod>/<version>/nix.packages` can remain as lightweight metadata if useful, but it should feed flake generation or a centralized resolver instead of being interpreted independently in many places.

4. Mod `install` commands should not be the primary way to create the dev environment.
   They should verify prerequisites, perform app-specific setup, or install optional runtime assets. They should not be responsible for reconstructing the base toolchain.

5. Offline and local-cache operation must be first-class.
   The system should work cleanly with a pinned lock file, cached store paths, `--offline`, and optional local `nixpkgs` references.

6. Development and build commands should use Nix tools, not host tools.
   If a mod runs through `./dialtone_mod`, its `go`, `ssh`, and similar tool invocations should resolve to Nix-store binaries from the active flake shell.

## Recommended Architecture

## 1. Root Flake Becomes the Only Environment Authority

The root flake should define:

- one default dev shell for general repo work
- one dev shell per mod/version where a focused environment is useful
- one app per mod/version command surface where a stable entrypoint is useful

Current shape:

- `devShells.default`
- `devShells.repl-v1`
- `devShells.ssh-v1`
- `apps.dialtone-mod`
- `apps.repl-v1`
- `apps.ssh-v1`

Planned next additions:

- `devShells.chrome-v1`
- `devShells.mosh-v1`
- `devShells.tsnet-v1`
- `apps.chrome-v1`

The key point is that `flake.lock` then becomes the real environment lock for mod development.

## 2. `dialtone_mod` Should Use `nix develop path:$REPO_ROOT`

Instead of:

```bash
nix shell nixpkgs#bashInteractive nixpkgs#git nixpkgs#go_1_24 ...
```

prefer:

```bash
nix develop path:$REPO_ROOT#default --command ./dialtone_mod ...
```

or, for focused shells:

```bash
nix develop path:$REPO_ROOT#repl-v1 --command ./dialtone_mod repl v1 test
```

This removes reliance on the caller's `flake:nixpkgs` registry alias and keeps command resolution pinned to the repo lock.

## 3. Generate Remaining Per-Mod Shells from One Central Rule

The mod metadata model should be centralized.

Recommended input:

- shared base packages from the root flake
- optional per-mod additions from `src/mods/<mod>/<version>/nix.packages`
- optional platform selectors already supported in the manifest format

Recommended behavior:

- a single flake helper reads or mirrors those manifests
- the helper produces per-mod package lists
- dev shells and apps are generated from that data

This preserves the nice part of the current `nix.packages` files without forcing every remaining mod CLI to interpret them separately.

## 4. Split Environment Concerns from Runtime Concerns

There are two different jobs that are currently blurred together:

- constructing a reproducible development shell
- performing mod-specific operational setup

These should be separated.

Examples:

- `repl v1 install` should verify or initialize REPL-specific state only
- `mosh v1 install --ensure` may still make sense as an optional runtime convenience if it intentionally installs profile-level binaries
- `chrome v1 install` may verify browser/runtime availability, but should not need to reconstruct the base shell itself

The base rule should be:

- environment from flake
- operational setup from mod CLI

## 5. Add `direnv` for the Common Path

For daily development speed, the repo should support:

```bash
direnv allow
```

with an `.envrc` equivalent to:

```bash
use flake
```

or a small wrapper around the repo flake if extra environment exports are needed.

This matters because the biggest productivity win is not theoretical purity. It is avoiding repeated shell startup and repeated environment reconstruction while moving between mods.

## 6. Keep Offline and Local-Source Support Explicit

The system should support:

- locked flake inputs
- local `path:` flake references when needed
- `--offline` when store data is already available
- optional binary cache use

Practical support knobs:

- `DIALTONE_NIX_OFFLINE=1`
- `DIALTONE_NIXPKGS_FLAKE=path:/path/to/nixpkgs`

Those are useful escape hatches, but in the target design they should be secondary to the locked repo flake, not part of the normal workflow.

## Mod-Specific Compatibility Notes

### `repl v1`

`repl v1` should fit the new model directly.

Current shape:

- runtime commands live in `src/mods/repl/v1`
- lifecycle commands live in `src/mods/repl/v1/cli`
- `nix.packages` is just the shared Go toolchain set

Implications for the migration:

- `repl-v1` should get a dedicated flake dev shell
- `./dialtone_mod repl v1 run|logs|version` should work inside that shell with no extra ad hoc `nix shell`
- `repl v1 install` should become a lightweight verification/init command, not a shell-construction command
- `repl v1 test|build|format` should rely on the flake shell as the default environment source

What must remain true:

- local runtime log paths still resolve relative to the repo
- `./dialtone_mod repl v1 run --once hello` works from a clean flake-backed shell
- `./dialtone_mod repl v1 test` works without consulting the caller's global `nixpkgs` registry

### `ssh v1`

Current shape:

- the mod is implemented in a single root package, not a split root/`cli` arrangement
- it runs only inside the `ssh-v1` flake shell
- it requires `DIALTONE_SSH_BIN` from that shell
- it rejects `--nixpkgs-url`

Implications for the migration:

- the normal `ssh v1` path now works only from the root flake or `ssh-v1`
- this is intentional because the goal is to prevent host-tool fallback
- the migration pattern for future mods should follow this stricter rule, not the older compatibility rule

Recommended target behavior:

- `./dialtone_mod ssh v1 ...` enters `ssh-v1` when needed
- inside that shell, `ssh v1` executes only the Nix `ssh` binary from `/nix/...`
- no nested-Nix override path remains
- shell contents: `ssh-v1` must include `openssh` in addition to the shared base packages

What must remain true:

- `./dialtone_mod ssh v1 --host gold --dry-run` still shows a valid command path
- `./dialtone_mod ssh v1 test --host gold` still works from a clean machine after entering the flake shell
- the printed `ssh` path comes from `/nix/store/...`, not host `/usr/bin/ssh`
- the migration does not break the current host/alias behavior from `env/mesh.json`

## Proposed Command Model

### Normal Repo Work

```bash
nix develop
./dialtone_mod repl v1 test
./dialtone_mod chrome v1 service status
```

### Focused Mod Work

```bash
nix develop .#repl-v1
./dialtone_mod repl v1 run --once hello
```

### Fully Explicit Reproducible Command Run

```bash
nix develop path:$PWD#repl-v1 --command ./dialtone_mod repl v1 test
```

### App Entry

```bash
nix run .#repl-v1 -- run --once hello
```

## Implementation Plan

### Phase 1: Normalize the Flake Surface

1. Add a proper `apps.dialtone-mod` entry to the root flake.
2. Add per-mod dev shells for the actively used mods first:
   - `repl-v1`
   - `ssh-v1`
   - `chrome-v1`
   - `mod-v1`
   - `mosh-v1`
   - `tsnet-v1`
3. Ensure those shells include the repo-level exports currently performed in the default `shellHook`.
4. Export explicit tool paths such as `DIALTONE_GO_BIN` and `DIALTONE_SSH_BIN` from those shells.

Deliverable:

- developers can use the root flake alone to enter both general and focused mod shells

### Phase 2: Refactor `dialtone_mod` to Be Flake-First

1. Replace the primary `nix shell nixpkgs#...` path with `nix develop path:$REPO_ROOT#<shell>`.
2. Ensure the wrapper selects a default shell or a mod-specific shell deterministically.
3. Use the flake shell as the only supported environment for migrated mods.

Deliverable:

- `./dialtone_mod` no longer depends on the user's global `nixpkgs` registry alias for normal operation

### Phase 3: Simplify Mod `install` Commands

1. Remove duplicated base-shell construction logic from mod CLIs.
2. Keep only:
   - verification of app/runtime prerequisites
   - optional profile/system installation where intentionally required
   - mod-local initialization
3. Document the narrowed meaning of `install` in `src/mods/README.md`.
4. Apply this first to `repl v1`; `ssh v1` does not have a separate lifecycle CLI, but it should still follow the “Nix shell only, no host tool fallback” rule.

Deliverable:

- mod CLIs stop rebuilding the dev shell on their own

### Phase 4: Add `direnv` Support

1. Add `.envrc` or equivalent documentation.
2. Make sure `nix develop` and `direnv` expose the same repo variables.
3. Validate the path behavior for both macOS and Linux.
4. Document the persistent-shell workflow explicitly for Codex and human users.

Deliverable:

- entering the repo automatically provides a warm shell for repeated mod work

### Phase 5: Optional Cache Improvements

1. Add documentation for local store reuse and offline mode.
2. If cold-start speed across machines matters, add a binary cache strategy.
3. Keep this phase optional because it improves speed, not correctness.

Deliverable:

- better cold-start behavior without changing the core contract

## Acceptance Criteria

The migration is complete when all of the following are true:

1. `./dialtone_mod` uses the root flake by default for mod execution.
2. A clean machine can clone the repo, run `nix develop`, and immediately use active mods reproducibly.
3. The normal path does not depend on `flake:nixpkgs` global registry resolution.
4. At least the main active mods have dedicated flake-backed dev shells.
5. `repl v1` works fully from a flake-backed shell for `run`, `logs`, `test`, and `build`.
6. `ssh v1` works fully from a flake-backed shell for `run` and `test`, and uses only the Nix `ssh` binary from the active shell.
7. Migrated mods do not use host `go`, host `ssh`, or similar host tools for development/build/exec paths.
8. Mod `install` commands no longer duplicate generic Nix shell construction, except for non-migrated mods that have not yet been refactored.
9. Offline execution works cleanly after inputs are cached locally.
10. Repo documentation explains the difference between:
   - dev environment entry
   - mod command routing
   - optional runtime installation/setup
   - persistent shell workflow for repeated work

## Risks

1. Overfitting per-mod shells too early.
   If every mod gets a special shell immediately, the flake may become harder to maintain than the current system.

2. Mixing environment and provisioning again.
   If `install` remains vague, the new design will still drift back toward duplicated bootstrap logic.

3. Hidden platform drift.
   Some mod dependencies differ on Darwin vs Linux; those cases need to remain explicit in the centralized model.

4. Startup cost outside persistent shells.
   `./dialtone_mod` still has to enter `nix develop` when called from outside a shell. That is correct, but users need a documented persistent-shell workflow for faster iteration.

## Current Status

Already done:

1. add `apps.dialtone-mod`
2. add `devShells.repl-v1`
3. add `devShells.ssh-v1`
4. switch `dialtone_mod` to `nix develop` against the root flake for those two mods
5. remove the old ad hoc REPL shell path in favor of the flake shell
6. remove the old nested-Nix SSH path and require Nix-shell `ssh`
7. export `DIALTONE_GO_BIN` and `DIALTONE_SSH_BIN`

## Recommended Next Slice

The best next implementation slice is:

1. add `devShells.chrome-v1`
2. add `devShells.mosh-v1`
3. add `devShells.tsnet-v1`
4. move those mods off ad hoc `nix shell nixpkgs#...`
5. update repo docs for `direnv` and persistent-shell development
6. keep using the same strict rule: migrated mods use Nix tools only, not host tools

That slice extends the proven model instead of introducing a second one.

## Success Metric

The practical success metric is simple:

- a developer can clone the repo, enter one pinned shell, and iterate across mods without fighting Nix, registry lookups, or repeated environment bootstrap code

If that is not true, the design is still too indirect.
