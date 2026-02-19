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
- `./dialtone.sh` starts a REPL with a `DIALTONE>` 
- see [DIALTONE.md](./docs/dialtone/DIALTONE.md).

The REPL accepts commands from user roles (e.g. `USER-1>`), including plugin commands such as the [robot plugin](src/plugins/robot/README.md) for dev, deploy, and telemetry.

**Example (robot plugin):**
```text
USER-1> robot dev src_v1
DIALTONE> Starting robot dev (mock data)...
DIALTONE:robot> Vite at http://127.0.0.1:3000
DIALTONE:robot> Chrome launched. Use 9:Mode to switch views.
USER-1> robot test src_v1
DIALTONE> Running robot tests...
DIALTONE:robot> [PASS] headless tests complete.
DIALTONE> Process exited with code 0.
```

## 2. Code Stack
The following components form the core architecture of Dialtone. LLM agents should treat the **DAG Plugin** as the canonical source of truth for UI and interaction patterns.

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
- `./dialtone.sh` and `.\dialtone.ps1` start a REPL with `DIALTONE>`
- `src/dev.go` is the main entry for the Command Line Interface (CLI)
- golang is used to scaffold the rest of the code base
- `src/plugins/` contains the plugins for the program
- plugins are the main way to extend `DIALTONE>`
- `env/.env` contains the environment variables for the program

- `src/libs/` contains the shared libraries for the program
- `src/plugins/` contains the plugins for the program
- `src/skills/` contains the skills for the program
- `src/tools/` contains the tools for the program
- `src/tests/` contains the tests for the program
- `src/examples/` contains the examples for the program
- `src/docs/` contains the documentation for the program
- `src/examples/` contains the examples for the program
- writing code with `DIALTONE>`



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
