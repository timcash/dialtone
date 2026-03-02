# <mod-name> Mod (`vN`)

This file is the short, LLM-first contract for this Dialtone mod.

## Command Contract
Use this command shape for all mod operations:

```bash
./dialtone.sh <mod-name> vN <command> <args> [options]
```

Standard commands every mod should expose:

```bash
./dialtone.sh <mod-name> vN install
./dialtone.sh <mod-name> vN format
./dialtone.sh <mod-name> vN lint
./dialtone.sh <mod-name> vN build
./dialtone.sh <mod-name> vN test
./dialtone.sh <mod-name> vN deploy
```

## Purpose
- What this mod does:
- Who/what depends on it:
- Primary runtime targets (local/remote/mesh):

## Layout
```text
src/mods/<mod-name>/
  README.md
  scaffold/main.go
  vN/
    go/
    cmd/
    test/
      cmd/main.go
      01_.../suite.go
      02_.../suite.go
```

## Scaffold Rule
- Keep `scaffold/main.go` thin.
- Put operational logic in `vN` (for example `vN/go`).

## Command Behavior
### `install`
- Installs runtime/build dependencies.
- Must be safe to run repeatedly.

### `format`
- Formats source files only.
- Must not change behavior.

### `lint`
- Runs static checks and style checks.
- Non-zero exit on violations.

### `build`
- Produces versioned build artifact(s).
- Document output path(s):

### `test`
- Runs `vN/test/cmd/main.go` as the orchestrator.
- Support optional filters via flags when useful.

### `deploy`
- Deploys build artifact(s) to target host(s).
- Must document auth/host assumptions and rollback strategy.

## Environment
- `.env` keys used by this mod:
- Required vars:
- Optional vars:
- Defaults:

## Paths and Config
- Use Dialtone `config` plugin runtime/path resolution.
- Avoid hardcoded absolute paths.

## Logging
- Use `logs` plugin for operational output.
- Include enough context to debug failures remotely.

## Testing Notes
- Fast local smoke test command:
- Full test command:
- Remote/mesh test command:

## Deploy Notes
- Example deploy command:
- Health check command:
- Verification/rollback command:

## Troubleshooting
- Common failure:
  - Cause:
  - Fix:

## Changelog
- `vN`:
  - Initial release notes.
