# [`DIALTONE`](https://dialtone.earth) `https://dialtone.earth`
> A Virtual Librarian for learning, and civic coordination.

## What is Dialtone?
Dialtone is a small program that runs on computers, phones, and robots. It is built for small communities that need practical tools for learning, planning, and operations.

- **Coordination:** Starts a virtual librarian for civic coordination and education.
- **Visualization:** Builds interactive 3D spaces to work with users.
- **Communication:** Uses mesh radios and local networks for peer-to-peer robot/human communication.
- **Extensibility:** Robust plugin system for safely adding new skills and models.
- **Integration:** Combines CAD, BIM, ERP, and GIS workflows into a single unified pattern.

![dialtone](./src/plugins/www/screenshots/summary.png)

## 1. REPL / Chat Interface (Target)
- `./dialtone.sh` and `./dialtone.ps1` start a simple `DIALTONE>` dialog.
- First-run flow: `DIALTONE>` asks for consent before installing only the minimum Go runtime + bootstrap files needed for `dev.go`.
- After bootstrap, `DIALTONE>` can install plugins and connect to swarm, VPN, and NATS.
- see [DIALTONE.md](./docs/dialtone/DIALTONE.md).

The REPL accepts commands from user roles (e.g. `USER-1>`), including robot development and deployment tasks.

**Example:**
```text
USER-1> ./dialtone.sh
DIALTONE> I can bootstrap dev mode. Install minimal Go runtime + bootstrap files now? [yes/no]
USER-1> yes
DIALTONE> Installing minimal runtime and loading `src/dev.go`...
DIALTONE> Bootstrap complete. New capabilities available: plugin install, swarm, vpn, nats, task DAG.
USER-1> @DIALTONE plugin install robot
USER-1> @DIALTONE swarm connect
USER-1> @DIALTONE vpn up
USER-1> @DIALTONE task start robot-nav-update-v3
DIALTONE> Task selected: [robot-nav-update-v3]. Invite roles `LLM-CODE`, `LLM-TEST`, `LLM-OPS`?
USER-1> yes
LLM-CODE> [Edit src/plugins/robot/nav_controller.py: improve obstacle avoidance near loading docks]
LLM-TEST> @DIALTONE pytest src/plugins/robot/tests/test_nav_controller.py
DIALTONE> Request received. Sign with `@DIALTONE task --sign robot-nav-update-v3` to run.
USER-1> @DIALTONE task --sign robot-nav-update-v3
LLM-OPS> @DIALTONE task --sign robot-nav-update-v3
DIALTONE> Signatures verified. Running command via PID 4512...
DIALTONE:4512> [PASS] test_nav_controller.py::test_dock_approach
DIALTONE:4512> [PASS] test_nav_controller.py::test_obstacle_recovery
DIALTONE> Process 4512 exited with code 0.
DIALTONE> DAG updated. Logs and artifacts published over swarm + VPN + NATS.
```

## 2. Code Stack
The following components form the core architecture of Dialtone. LLM agents should treat the **DAG Plugin** as the canonical source of truth for UI and interaction patterns.

### Three-Layer Tech Stack
1. **Shell Layer**: `./dialtone.sh` is designed to stay simple, run from anywhere, and start `DIALTONE>` (the virtual librarian interface).
2. **Dev Layer**: `src/dev.go` provides scaffolding that adds structure around a heterogeneous codebase that can include many languages.
3. **Plugin Layer**: the plugin system is designed to make complexity composable by linking plugin behaviors back to `src/dev.go`.

### Bootstrap to Collaboration Flow
1. Start from `./dialtone.sh` or `./dialtone.ps1`.
2. Confirm minimal bootstrap install (runtime + basic Go files only).
3. Activate `dev.go` command routing.
4. Install only needed plugins and connect transport layers (swarm, VPN, NATS).
5. Collaborate in `DIALTONE>` with `USER-*` and `LLM-*` roles on a shared DAG of tasks and logs.

### [DAG Plugin](./src/plugins/dag/README.md)
The primary implementation of the Directed Acyclic Graph engine. It defines the standard for section lifecycle, mode-form controls, and 3D stage interactions.

### [Template Plugin](./src/plugins/template/README.md)
A reusable starter kit for new plugins. Use this to scaffold new functionality while maintaining compatibility with the Dialtone lifecycle and test runners.

### [Logs Plugin](./src/plugins/logs/README.md)
NATS-first logging across plugins and services. Producers publish to NATS; readers attach (CLI `--stdout`, file listener, browser). See [src/plugins/logs/src_v1/README.md](src/plugins/logs/src_v1/README.md) for CLI and dev flow.

### [UI Library (ui_v2)](./src/libs/ui_v2/README.md)
The shared toolkit for building Dialtone interfaces. It handles section management, global menus, and overlay coordination.

### [Test Library (test_v2)](./src/libs/test_v2/README.md)
A specialized browser orchestration library for deterministic UI testing. It provides ARIA-driven actions, automated reporting, and screenshot capture.

## 3. Getting Started
The easiest way to get started is `https://dialtone.earth`.

### Clone and Run
```bash
git clone https://github.com/dialtone/dialtone.git
cd dialtone
./dialtone.sh
```

### Run on Windows
```powershell
./dialtone.ps1 --help
```

### Run on Linux / WSL / macOS
```bash
./dialtone.sh --help
```



## How the code base is organized
### Entry points
- `./dialtone.sh`, `./dialtone.ps1`, and `./dialtone.cmd` are thin wrappers that start the `DIALTONE>` workflow.
- `src/dev.go` is the main CLI scaffold/dispatcher for command routing.

### Core source layout
- `src/core/`: shared core CLI/runtime packages.
- `src/plugins/`: plugin modules and plugin CLIs (primary extension surface).
- `src/libs/`: reusable libraries used across plugins and CLI flows.
- `src/cmd/`: command-oriented packages and support entrypoints.
- `docs/`: project docs, protocol docs, and examples/transcripts.
- `skills/`: skill definitions used by the system.
- `env/.env`: environment configuration for local/runtime setup.

### Three-layer mental model
1. **Shell Layer**: wrapper scripts (`./dialtone.sh` etc.) keep startup simple from anywhere.
2. **Dev Layer**: `src/dev.go` provides structure across a heterogeneous, multi-language codebase.
3. **Plugin Layer**: plugins compose higher-level behavior and link back through the `src/dev.go` command router.

### Suggested format to document the codebase
Use the same mini-template for each top-level folder or plugin:
- **Purpose**: what this area is responsible for.
- **Entrypoint**: the main command/file to start from.
- **Key files**: 3-5 important files/directories.
- **How to run/test**: exact commands for development and verification.
- **Dependencies**: external services, env vars, and related plugins.
- **Ownership/status**: who maintains it and current maturity (alpha/beta/stable).



## Plugin README.md
Every plugin must include a `README.md` at its plugin root (`src/plugins/<plugin>/README.md`).

Use the shared template:

- [README_TEMPLATE.md](./docs/templates/README_TEMPLATE.md)


## TODO

### build DIALTONE> cli 
- start with a simple dailtone.sh scripted interatcion
- update core with the new dialtone process manager to spawn subtones
- log all interactions in swarm autolog
- integrate with swarm for the dag task database
- show a workflow of starting on a new computer for the first time and getting things installed

### add key management tools
- allow USER> and LLM-*> to send a password and get authed with DIALTONE>

### integrate nix as the package manager
- show a full ./dialtone.sh workflow as a starting point
- start with install from just the `dialtone.sh` or `dialtone.ps1` wrapper
- get nix installed via the `./dialtone.sh nix install` command
- show a plugin install via `./dialtone.sh plugin install <plugin-name>`
