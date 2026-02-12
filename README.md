# [`DIALTONE`](https://dialtone.earth)
> IMPORTANT: This should sound like something from a scirence fiction novel about the near future of civic technology.

![dialtone](./dialtone.jpg)

## Website
[dialtone.earth](https://dialtone.earth)

## Getting Started

### Windows
Run commands using the `dialtone.cmd` wrapper (no PowerShell policy issues):
```cmd
.\dialtone start
.\dialtone task create my-new-task
```

### Linux / WSL / macOS
Run commands using `dialtone.sh`:
```bash
./dialtone.sh start
./dialtone.sh task create my-new-task
./dialtone.sh help      # same as ./dialtone.sh --help
```

## Shell Process Management

`./dialtone.sh` now uses explicit process management. It no longer auto-kills child processes on shell exit.

### Key behavior
- No automatic wrapper cleanup trap for child processes.
- Long-running commands can be inspected/stopped explicitly.
- Logs and process metadata are tracked under `.dialtone/run/` for top-level and nested `./dialtone.sh` commands while running.

### List processes
```bash
# All running ./dialtone.sh processes (including nested) (default)
./dialtone.sh ps
./dialtone.sh ps all

# Tracked processes only (top-level + nested)
./dialtone.sh ps tracked

# Tree view: dialtone.sh + go/bun children
./dialtone.sh ps tree
```

### Manage tracked processes
```bash
# List tracked processes (top-level + nested)
./dialtone.sh proc ps

# Stop a tracked process by key
./dialtone.sh proc stop dag_dev_src_v2

# Tail tracked wrapper log for key
./dialtone.sh proc logs dag_dev_src_v2
```

### Kill processes
```bash
# Kill one process tree by PID
./dialtone.sh kill 48969

# Kill all running ./dialtone.sh process trees
./dialtone.sh kill all
```

### Example: DAG dev workflow
```bash
# Start dev server + browser session
./dialtone.sh dag dev src_v2

# In another terminal, inspect running processes
./dialtone.sh ps tracked
./dialtone.sh ps tree

# Stop explicitly when done
./dialtone.sh proc stop dag_dev_src_v2

# Or terminate by PID / all wrappers
./dialtone.sh kill <pid>
./dialtone.sh kill all
```

### Notes
- Some plugins also write plugin-specific logs (for example: `src/plugins/dag/src_v2/dev.log`).
- `./dialtone.sh help` and `./dialtone.sh --help` are equivalent.
- `--timeout` is deprecated and ignored.
- `--grace` still controls how long `proc stop` waits before SIGKILL.

## What is Dialtone?
Dialtone is a public information system for civic coordination and education. It connects people, tools, and knowledge to solve real-world problems.

- **A Virtual Librarian**: Guides students and builders through mathematics, physics, and engineering using real-world examples.
- **Civic Coordination**: Tools for city councils and teams to plan, budget, and track public projects.
- **Shared Infrastructure**: A network of publicly owned robots, radios, and sensors that anyone can learn from and build upon.
- **Live Operations**: Real-time maps and status dashboards for tracking active missions and deployed hardware.
- **Open Marketplace**: Connects operators with parts, services, and training needed to keep systems running.
- **Secure Identity**: Cryptographic tools for managing keys, identity, and access to shared resources.

## Who uses Dialtone
- **Students**: Use the virtual library to learn mathematics, physics, and engineering through guided problems tied to real systems.
- **City councils and civic teams**: Plan new public spaces, coordinate contractors, and monitor ongoing work.
- **Builders and operators**: Assemble radio and robot kits, run missions, and ship improvements.
- **Developers**: Contribute documentation, tests, and code to the platform.

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
