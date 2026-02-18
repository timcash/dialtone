# [`DIALTONE`](https://dialtone.earth) `https://dialtone.earth`
> A shared software system for learning, coordination, and real-world community work.

![dialtone](./src/plugins/www/screenshots/summary.png)

## REPL / Chat Interface (Target)
`./dialtone.sh` should move to a simple text-chat REPL with a `DIALTONE>` prompt, following [DIALTONE.md](./DIALTONE.md) and [RLM_DIALTONE.md](./RLM_DIALTONE.md).  
Users and agents submit role-based commands (for example `USER-1>`), and command output is streamed back as `DIALTONE:PID:>`.

Example:
```text
USER-1> ps all
DIALTONE> Request received. Sign with `@DIALTONE task --sign current-task` to run.
USER-1> @DIALTONE task --sign current-task
DIALTONE> Signatures verified. Running command via PID 4512...
DIALTONE:4512> PID  USER  CMD
DIALTONE:4512> 4512 user  dialtone
DIALTONE> Process 4512 exited with code 0.
```

## `PERSONA>` Model
Dialtone uses explicit speaking roles in the REPL so every line has a clear source.

- `DIALTONE>`: the core orchestrator process.
- `DIALTONE:PID>`: a Dialtone subtone/subprocess stream (per running task/process).
- `USER-1>` (and other `USER-*` roles): cryptographically signed user identities.
- `LLM-*>`: dynamically created LLM agent subprocesses orchestrated by `DIALTONE>`, similar to subtones but specialized for reasoning/planning/tool use.

This makes command intent, authorization, and runtime output easy to audit in one shared session.

## Logs (plugin: `plugins/logs/src_v1`)
Dialtone has a logs plugin at `src/plugins/logs/src_v1` to standardize local/dev/prod logs across plugins and services.

Planned log levels:
- `--quiet` (minimal output)
- `--verbose` (operator-friendly detail)
- `--debug` (full diagnostic detail)

Goal: same log format everywhere, with strong defaults for command history, process lifecycle, errors, and API call summaries.

## Telemetry and Trace (TODO)
Dialtone should add telemetry/tracing so operators can follow activity across the network and inspect end-to-end latency.

Planned behavior:
- `--trace` turns on distributed tracing for requests, function calls, and API hops.
- Trace output links events to logs so one request can be followed across services.
- Telemetry captures live production signals like CPU stats, memory, latencies, versions, and other runtime metrics needed for analysis.

## Getting Started
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

## What Dialtone Is
Dialtone is a small program that runs on computers, phones, and robots. It is built for small communities that need practical tools for learning, planning, and operations.

- It acts like a virtual librarian for civic coordination and education.
- It can build interactive 3D spaces to work with users.
- Mesh radios and local networks help people and robots communicate.
- It has a plugin system so teams can extend functionality safely.
- It can adapt to new tasks by distilling and fine-tuning focused sub-models.
- It combines systems like CAD, BIM, ERP, GIS, and related workflows into one pattern.
- It includes plugins for testing, development, and deployment.
- It supports a robotics and manufacturing marketplace for parts, services, and data.
- It is open to contributors who want to build skills in robotics, math, manufacturing, and more.

## Code Stack
### DAG Plugin
The DAG plugin is the main source of truth for current UI/interaction patterns, section naming, overlays, and test flow. Agents should start here to understand how the system is expected to behave at runtime.  
README: [src/plugins/dag/README.md](src/plugins/dag/README.md)

### Template Plugin
The template plugin is the reusable starter for building new plugins with the same lifecycle, section model, and dev/test commands. It mirrors production patterns in a simpler package.  
README: [src/plugins/template/README.md](src/plugins/template/README.md)

### Logs Plugin (`logs` / `src_v1`)
The logs plugin standardizes logging across plugins and services, unifies browser and backend logs, and supports NATS topics and xterm streaming.  
README: [src/plugins/logs/src_v1/README.md](src/plugins/logs/src_v1/README.md)

### UI Library (`ui_v2`)
`ui_v2` provides shared primitives for section lifecycle, menu handling, overlay/underlay wiring, and URL-driven navigation. Plugin UIs should compose these building blocks instead of re-implementing them.  
README: [src/libs/ui_v2/README.md](src/libs/ui_v2/README.md)

### Test Library (`test_v2`)
`test_v2` provides browser/session orchestration, aria-label-driven actions, logging, and screenshot/report helpers for deterministic UI tests. Use it to keep plugin validation behavior consistent across the repo.  
README: [src/libs/test_v2/README.md](src/libs/test_v2/README.md)

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

## Who uses Dialtone
- Manufacturers building production and sensor systems.
- Students learning math, physics, and engineering with real examples.
- Civic teams planning public projects and tracking progress.
- Builders and operators assembling kits and running field work.
- Developers writing code, docs, and tests.
- Researchers running experiments and monitoring live data.

### README.md
Every plugin must include a `README.md` at its plugin root (`src/plugins/<plugin>/README.md`).

Use the shared template:

- [README_TEMPLATE.md](./README_TEMPLATE.md)

## DIALTONE example session log
```text
USER-1> @DIALTONE npm run test
DIALTONE> Request received. Sign with `@DIALTONE task --sign test-task`...
USER-1> @DIALTONE task --sign test-task
DIALTONE> Signatures verified. Running command via PID 4512...

DIALTONE:4512> > dialtone@1.0.0 test
DIALTONE:4512> > tap "test/*.js"
DIALTONE:4512> [PASS] test/basic.js
DIALTONE:4512> Tests completed successfully.
DIALTONE:4512> [EXIT] Process exited with code 0.
```
