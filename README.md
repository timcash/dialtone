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
```

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
