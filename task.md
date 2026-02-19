# Task: Bootstrap `DIALTONE>` REPL + Dev Scaffold Workflow

## Goal
Implement a top-level bootstrap experience where `./dialtone.sh` and `./dialtone.ps1` start a simple `DIALTONE>` + `USER-1>` dialog, then progressively enable development capabilities:

1. Introduce Dialtone and its core functions.
2. Accept user input via `USER-1>`.
3. On `dev install`, install the latest stable Go runtime and bootstrap `dev.go`.
4. Enable plugin-oriented workflows (for example, `@DIALTONE robot install src_v1`).
5. Prepare for plugin-only downloads (without full git clone) from GitHub main/branch.

---

## Product Behavior (Target)

### Bootstrap Interaction
- `./dialtone.sh` (no args) starts REPL mode.
- Prompt format:
  - Output: `DIALTONE> ...`
  - Input: `USER-1> ...`
- Built-in commands:
  - `help`
  - `exit` / `quit`
  - `dev install`

### `dev install` Behavior
- Installs latest stable Go runtime into `DIALTONE_ENV/go`.
- Validates managed Go binary is runnable.
- Boots `src/cmd/dev/main.go` to initialize command scaffold (help/registry path).
- Returns user to REPL with confirmation that plugin commands are available.

### Plugin Path
- `USER-1> @DIALTONE robot install src_v1` should execute from REPL through standard command routing.
- Plugin install should prepare local dependencies for that plugin version.
- Commands are executed directly via subtone streaming.

**Example dialog (`robot install src_v1` via subtone):**
```text
USER-1> @DIALTONE robot install src_v1
DIALTONE> Request received. Spawning subtone for robot install...
DIALTONE> Signatures verified. Spawning subtone subprocess via PID 5821...
DIALTONE> Streaming stdout/stderr from subtone PID 5821.
DIALTONE:5821:> >> [Robot] Install: src_v1
DIALTONE:5821:> >> [Robot] Checking local prerequisites...
DIALTONE:5821:> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:5821:> >> [Robot] Installing Go dependencies for src_v1
DIALTONE:5821:> >> [Robot] Install complete: src_v1
DIALTONE> Process 5821 exited with code 0.
```

---

## Implementation Outline for LLM Agent

### Phase 1 - REPL Foundation
- [x] Add interactive REPL startup in `dialtone.sh` when no command is supplied.
- [x] Add `DIALTONE>` output helper and `USER-1>` input loop.
- [x] Add `help`, `exit`, and fallback command forwarding (`./dialtone.sh <command>`).

### Phase 2 - Dev Bootstrap
- [x] Add `dev install` handling in REPL.
- [x] Update Go installer to support latest stable resolution (`--latest`).
- [x] Run `dev.go` help/boot command after install to verify scaffold route.

### Phase 3 - Command Routing Consistency
- [ ] Mirror REPL bootstrap behavior in `dialtone.ps1`.
- [ ] Standardize UX messages between bash and PowerShell wrappers.
- [ ] Add explicit success/failure state messages with next suggested commands.

### Phase 4 - Plugin-Only Distribution (Design + Prototype)
- [ ] Define per-plugin package layout (zip/tar) excluding heavy assets (images/screenshots).
- [ ] Build a release artifact flow:
  - `plugin pack <name> [version]`
  - attach plugin package to GitHub release (main + optional branch channel).
- [ ] Add `plugin install <name> [version|branch]` that downloads/extracts package without git clone.
- [ ] Validate dependency bootstrapping from extracted plugin package only.

### Phase 5 - DAG Collaboration Workflow
- [ ] Add documented flow: `USER-*` + `LLM-*` roles collaborate in a DAG.
- [ ] Ensure logs/artifacts can be published over Swarm + VPN + NATS.
- [ ] Add task orchestration examples in docs and smoke tests.

---

## Test Plan

### A. REPL Startup
- Run `./dialtone.sh` from repo root.
- Confirm intro text appears with `DIALTONE>`.
- Confirm prompt is `USER-1>`.
- Confirm `help` and `exit` work.

### B. Latest Go Install
- In REPL: run `@DIALTONE dev install` or just `dev install`.
- Verify:
  - Installer resolves latest stable version.
  - `DIALTONE_ENV/go/bin/go version` is available.
  - `src/cmd/dev/main.go help` executes.

### C. Plugin Command Path
- In REPL: run `@DIALTONE robot install src_v1`.
- Verify command is accepted and run immediately.
- Verify subtone output streams as `DIALTONE:PID:>`.
- Verify expected dependency artifacts exist for robot plugin.

### D. Regression Checks
- Run existing direct command mode:
  - `./dialtone.sh help`
  - `./dialtone.sh go help`
  - `./dialtone.sh robot help`
- Ensure non-REPL paths still function.

### E. Future Distribution Validation
- Install plugin package on clean machine with no git repo.
- Verify plugin commands run after package extraction + dependency bootstrap.

---

## Open Decisions
- Should `dev install` be non-interactive by default, or prompt for confirmation first?
- Should "latest stable" be pinned after first install for reproducibility?
- Should plugin packages be attached to GitHub Releases, a package registry, or both?
- Which files are always excluded from plugin packages (images, recordings, test artifacts)?

---

## Suggested Next Commands
- `./dialtone.sh`
- `dev install`
- `@DIALTONE robot install src_v1`
- `./dialtone.sh go version` (or `./dialtone.sh go exec version`)
