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

The REPL accepts commands from user roles (e.g. `USER-1>`), including plugin commands such as the [robot plugin](src/plugins/robot/README.md) for dev, deploy, and telemetry.

**Example (robot plugin):**
```shell
$ ./dialtone.sh
DIALTONE> Virtual Librarian online. Type 'help' for commands, or 'exit' to quit.
USER-1> robot dev src_v1
DIALTONE> Starting robot dev (mock data)...
DIALTONE:41146> Vite at http://127.0.0.1:3000
DIALTONE:41146> Chrome launched
USER-1> robot test src_v1
DIALTONE> Running robot tests...
DIALTONE:41146> [PASS] headless tests complete.
DIALTONE> Process exited with code 0
USER-1> exit
DIALTONE> Goodbye
```

## 2. Code Stack
The following components form the core architecture of Dialtone. LLM agents should treat the **DAG Plugin** as the canonical source of truth for UI and interaction patterns.

### Three-Layer Tech Stack
1. **Shell Layer**: `./dialtone.sh` is a thin bootstrap script that ensures Go is installed and hands over execution to `src/dev.go`.
2. **Dev Layer**: `src/dev.go` is the main orchestrator and REPL engine. It routes commands to plugins and manages subtone processes.
3. **Plugin Layer**: plugins compose higher-level behavior and link back through the `src/dev.go` command router.

### Bootstrap to Collaboration Flow
1. Start from `./dialtone.sh` or `./dialtone.ps1`.
2. Confirm minimal bootstrap install (runtime + basic Go files only).
3. Activate `dev.go` command routing and interactive REPL.
4. Install only needed plugins and connect transport layers (swarm, VPN, NATS).
5. Collaborate in `DIALTONE>` with `USER-*` and `LLM-*` roles on a shared DAG of tasks and logs.

### [DAG Plugin](./src/plugins/dag/README.md)
The primary implementation of the Directed Acyclic Graph engine. It defines the standard for section lifecycle, mode-form controls, and 3D stage interactions.

### [Test Plugin](./src/plugins/test/README.md)
A specialized browser orchestration library for deterministic UI testing. It provides ARIA-driven actions, automated reporting, and screenshot capture.

### [UI Plugin](./src/plugins/ui/README.md)
The shared toolkit for building Dialtone interfaces. It handles section management, global menus, and overlay coordination.

### [Logs Plugin](./src/plugins/logs/README.md)
NATS-first logging across plugins and services. Producers publish to NATS; readers attach (CLI `--stdout`, file listener, browser).

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

## 4. LLM Agent Usage (`./dialtone.sh` + REPL)
Yes. Tools like Claude Code, Codex CLI, and Gemini CLI can use Dialtone in two ways:

### A) Direct command mode (non-REPL)
Run plugin/orchestrator commands directly:

```bash
./dialtone.sh go version
./dialtone.sh robot install src_v1
./dialtone.sh ps tracked
```

### B) REPL mode (interactive or scripted)
Start an interactive session:

```bash
./dialtone.sh
```

Use agent-style command routing in REPL:
- `USER-1>` enters commands.
- `@DIALTONE ...` tells Dialtone to run managed commands/subtones.
- `DIALTONE:PID> ` streams subprocess output.

### C) REPL automation for LLM agents (no extra tools required)
For deterministic automation, run REPL from stdin:

```bash
./dialtone.sh <<'EOF'
@DIALTONE dev install
@DIALTONE robot install src_v1
status
exit
EOF
```

### D) Interactive human workflow
This is what a real interactive session looks like:

```text
USER-1> @DIALTONE robot install src_v1
DIALTONE> Request received. Spawning subtone for robot install...
DIALTONE> Spawning subtone subprocess via PID 530013...
DIALTONE> Streaming stdout/stderr from subtone PID 530013.
DIALTONE:530013> >> [Robot] Install: src_v1
DIALTONE:530013> >> [Robot] Install complete: src_v1
DIALTONE> Process 530013 exited with code 0.
USER-1> status
USER-1> exit
DIALTONE> Goodbye.
```

### Recommended agent pattern
1. Use direct commands for simple checks (`go version`, `ps`, `status`).
2. Use REPL for streaming plugin install/test output.
3. Use `env/test.env` for isolated runs so test dependencies install outside the repo (for example under `/tmp`).



## How the code base is organized
### Entry points
- `./dialtone.sh`, `./dialtone.ps1`, and `./dialtone.cmd` are thin wrappers that start the `DIALTONE>` workflow.
- `src/dev.go` is the main CLI orchestrator and REPL engine.

### Core source layout
- `src/`: core orchestrator logic and shared Go packages.
- `src/plugins/`: plugin modules and plugin CLIs (primary extension surface).
- `docs/`: project docs, protocol docs, and examples/transcripts.
- `skills/`: skill definitions used by the system.
- `env/.env`: environment configuration for local/runtime setup.

### Three-layer mental model
1. **Shell Layer**: wrapper scripts (`./dialtone.sh` etc.) keep startup simple and ensure Go is present.
2. **Dev Layer**: `src/dev.go` provides structure and routes commands to plugins.
3. **Plugin Layer**: plugins compose higher-level behavior and link back through the `src/dev.go` command router.




## Plugin README.md
Every plugin must include a `README.md` at its plugin root (`src/plugins/<plugin>/README.md`).

Use the shared template:

- [README_TEMPLATE.md](./docs/templates/README_TEMPLATE.md)

