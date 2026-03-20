# Mods Guide

This file is the working guide for the Dialtone mods system: how mods are
organized, how `./dialtone_mod` resolves them, how to work on Go mods with Nix,
and how LLM agents should route commands through Ghostty + tmux so the user can
see what is being run.

## Overview

Dialtone mods live under:

```text
src/mods/<mod-name>/<version>/
```

Examples:

```text
src/mods/ghostty/v1/
src/mods/tmux/v1/
src/mods/codex/v1/
src/mods/mod/v1/
```

The top-level launcher is:

```sh
./dialtone_mod <mod-name> <version> <command> [args]
```

Examples:

```sh
./dialtone_mod ghostty v1 help
./dialtone_mod shell v1 start
./dialtone_mod tmux v1 list
./dialtone_mod codex v1 status --session codex-view
./dialtone_mod mods v1 help
```

Notes:

- `mods` is an alias for the `mod` mod.
- Versions are explicit. `v1` and `v2` are separate command surfaces.
- `./dialtone_mod` runs `go run ./src/mods.go ...` under the repo's Nix-aware
  wrapper unless Nix is explicitly bypassed.

## Required Workflow

1. Use Ghostty as the interactive terminal UI.
2. Use a tmux session named `codex-view` inside Ghostty for interactive mod work.
3. Use Nix for all build, format, lint, and test work. Do not rely on host Go.
4. Use `./dialtone_mod` from the repo root, not ad-hoc `go run` commands, when
   exercising real mod behavior.
5. Prefer the repo's own control mods when operating the local session:
   - `ghostty v1`
   - `tmux v1`
   - `codex v1`
6. For the tested one-window/one-tab local workflow, use `codex-view:0:0` as
   the canonical tmux target unless you are explicitly debugging tmux layout.

## Version System

Each mod version is a stable CLI contract.

```sh
# List available mods and their versions.
./dialtone_mod help

# Work against a specific version explicitly.
./dialtone_mod ghostty v1 help
./dialtone_mod tmux v1 help
```

Rules:

- New compatible work should stay inside the existing version directory.
- Breaking CLI or behavior changes should go into a new version directory such
  as `v2`.
- Do not silently change `v1` semantics if downstream agents or scripts may
  already depend on them.

The launcher resolves entrypoints like this:

- `./dialtone_mod <mod> <version> <non-build-command>` runs the package in
  `src/mods/<mod>/<version>/` when present.
- `./dialtone_mod <mod> <version> install|build|format|test` prefers
  `src/mods/<mod>/<version>/cli/` when that package exists.

## Canonical Session Bootstrap

Use this to create the working Ghostty + tmux environment. Prefer the
single-command shell mod when you want the full local workflow.

```sh
# Preferred path: reset Ghostty to one window with one tab, attach codex-view,
# set the tmux proxy target, and launch Codex in one command.
./dialtone_mod shell v1 start
```

Manual path:

```sh
# Reset Ghostty to one window with one tab rooted at the repo.
# Keep AppleScript inside the Ghostty mod; use the mod CLI here.
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone

# Start or attach the canonical tmux session in the selected Ghostty terminal.
# `--focus` is harmless here and keeps the write target explicit.
./dialtone_mod ghostty v1 write --terminal 1 --focus "tmux new-session -A -s codex-view"

# Persist the default proxy target for future dialtone_mod commands.
# In the tested one-window/one-tab workflow, the canonical pane is
# `codex-view:0:0`.
./dialtone_mod tmux v1 target --set codex-view:0:0

# Launch Codex in the visible tmux session.
./dialtone_mod codex v1 start --session codex-view
```

## Working Through Ghostty And tmux

All LLMs should route mod work through the live Ghostty/tmux session whenever
the user should be able to see the commands being executed.

### Direct Injection

```sh
# Send a command to the live tmux pane explicitly.
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "pwd"

# Read back recent output from that pane.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40
```

### Set The Default tmux Target

Once the target is set, normal non-control `./dialtone_mod` commands are sent
into that tmux pane automatically.

```sh
# Set the default proxy target once.
./dialtone_mod tmux v1 target --set codex-view:0:0

# Show the current target.
./dialtone_mod tmux v1 target

# After this, non-control mod commands are injected into the tmux pane instead
# of running directly in the caller shell.
./dialtone_mod codex v1 status --session codex-view
./dialtone_mod repl v1 help

# Read the visible output from the tmux pane.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 80
```

Important behavior:

- `tmux v1`, `ghostty v1`, and `shell v1` are control mods and bypass the tmux proxy.
- Most other `./dialtone_mod` commands will be sent into the configured tmux
  pane once the target is set.
- This is the preferred mode for LLM agents because the user can see the exact
  commands arrive in Ghostty.

### Recommended LLM Pattern

```sh
# 1. Prefer the one-command bootstrap for a fresh local session.
./dialtone_mod shell v1 start

# 2. If you are following the manual path, make sure the tmux session exists.
./dialtone_mod ghostty v1 fresh-window --cwd /Users/user/dialtone
./dialtone_mod ghostty v1 write --terminal 1 --focus "tmux new-session -A -s codex-view"

# 3. Persist the canonical pane id for the tested layout.
./dialtone_mod tmux v1 target --set codex-view:0:0

# 4. Run normal non-control mod commands; they will be injected into tmux for
# visibility.
./dialtone_mod codex v1 status --session codex-view
./dialtone_mod repl v1 help

# 5. Read output back when needed.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 80
```

## Golang And Nix

All Go work must run with Nix-provided tooling.

### Enter A Nix Shell In The tmux Session

```sh
# Put the live tmux pane into the repo's default Nix shell.
./dialtone_mod tmux v1 shell --pane codex-view:0:0 --shell default

# Read the pane to confirm the shell switched.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 40
```

### Go Mod Development Workflow

For a Go-backed mod like `ghostty v1`, the normal work loop is:

```sh
# Change into the Go module root inside the repo.
cd /Users/user/dialtone/src

# Format the mod package and its tests.
gofmt -w mods/ghostty/v1/main.go mods/ghostty/v1/main_test.go

# Lint the package with go vet when the package is Go-based.
go vet ./mods/ghostty/v1

# Run unit tests for the mod package.
go test ./mods/ghostty/v1

# Build the package directly to verify it compiles cleanly.
go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1
```

If you want those commands to be visible to the user in Ghostty, inject them
through tmux:

```sh
# Run the whole Go validation sequence inside the live tmux pane.
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "cd /Users/user/dialtone/src && gofmt -w mods/ghostty/v1/main.go mods/ghostty/v1/main_test.go && go vet ./mods/ghostty/v1 && go test ./mods/ghostty/v1 && go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1"

# Read the result from the pane.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 120
```

### Using `./dialtone_mod` For Install, Format, Build, Test

When a mod has a `cli/` package, `./dialtone_mod <mod> <version> install|build|format|test`
prefers that CLI entrypoint. Use the mod's own help output to see which
commands it exposes.

```sh
# Discover the mod's own CLI surface.
./dialtone_mod mod v1 help
./dialtone_mod ghostty v1 help

# Example of a mod with a CLI entrypoint.
./dialtone_mod mod v1 format
./dialtone_mod mod v1 test
./dialtone_mod mod v1 build
```

For Go mods that do not expose `install|build|format|test` themselves, use the
Nix shell plus normal Go tools:

```sh
# Example for a Go mod without a dedicated build/test CLI.
cd /Users/user/dialtone/src
gofmt -w mods/ghostty/v1/main.go mods/ghostty/v1/main_test.go
go vet ./mods/ghostty/v1
go test ./mods/ghostty/v1
go build -o /tmp/dialtone-ghostty-v1 ./mods/ghostty/v1
```

### When Linting Needs More Than `go vet`

If a mod needs extra tools such as `staticcheck`, add them to the mod's
`nix.packages` file first so the tool is provided by Nix instead of the host.

```sh
# Example shape only; adjust the package if the mod actually adopts it.
printf '%s\n' 'nixpkgs#staticcheck' >> /Users/user/dialtone/src/mods/<mod>/<version>/nix.packages
```

## Full LLM Workflow For Go Mods

This is the expected end-to-end loop for an LLM agent working on a Go mod.

```sh
# 1. Start the visible local session with one command.
./dialtone_mod shell v1 start

# 2. Confirm the canonical tmux target.
./dialtone_mod tmux v1 target --set codex-view:0:0

# 3. Put the pane into the Nix shell.
./dialtone_mod tmux v1 shell --pane codex-view:0:0 --shell default

# 4. Inject formatting, linting, tests, and builds into the visible tmux pane.
./dialtone_mod tmux v1 write --pane codex-view:0:0 --enter "cd /Users/user/dialtone/src && gofmt -w mods/<mod>/<version>/*.go && go vet ./mods/<mod>/<version> && go test ./mods/<mod>/<version> && go build -o /tmp/<mod>-<version> ./mods/<mod>/<version>"

# 5. Read the output back and use it to decide the next edit.
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 160

# 6. Exercise the real mod behavior with ./dialtone_mod so the user can see it.
./dialtone_mod <mod> <version> help
./dialtone_mod <mod> <version> <command> [args]
./dialtone_mod tmux v1 read --pane codex-view:0:0 --lines 160
```

LLM rules:

- Use `./dialtone_mod shell v1 start` for fresh local session bootstrap unless
  you are debugging the lower-level Ghostty/tmux mods themselves.
- Prefer visible tmux-injected commands over invisible host-only execution.
- Set the tmux target early when you plan to use `./dialtone_mod` repeatedly.
- Use `tmux v1 read` after meaningful commands so the output is inspectable.
- Keep using Nix-backed tooling; do not switch to host Go, host linters, or
  host build chains mid-task.

## Creating Or Updating A Mod

New or existing mods should follow the versioned directory layout:

```text
src/mods/<name>/<version>/
```

Typical contents:

```text
src/mods/ghostty/v1/main.go
src/mods/ghostty/v1/main_test.go
src/mods/ghostty/v1/README.md
src/mods/ghostty/v1/nix.packages
src/mods/ghostty/v1/cli/
```

Guidelines:

- Put runtime behavior in `main.go` unless the package becomes large.
- If the package grows beyond a clean single-file implementation, split it by
  concern inside the same version directory.
- Use `main_test.go` or additional `*_test.go` files for unit tests.
- Add or update `nix.packages` when the mod needs extra tools.
- Keep the README accurate and runnable.

## Documentation Expectations

When updating a mod README, include:

1. `Quick Start`
2. `DIALTONE>`
3. `Dependencies`
4. `Test Results`

The `Quick Start` section should prefer shell code blocks with comments.

The `DIALTONE>` section should show realistic command/output examples so future
LLM agents can infer expected interaction patterns.

The `Dependencies` section should name other mods and versions the mod depends
on.

The `Test Results` section should track the most recent validation run with
metadata fields such as:

- `<timestamp-start>`
- `<timestamp-stop>`
- `<runtime>`
- `<ERRORS>`
- `<ui-screenshot-grid>`

## macOS Notes

- Ghostty's native AppleScript API is sufficient for window, tab, and terminal
  inspection, creation, splitting, focusing, and text input.
- `System Events` GUI scripting is only needed for fallback keystroke-style
  automation.
- If `System Events` automation is ever needed, grant the host app
  Accessibility and Automation permissions in macOS System Settings.
