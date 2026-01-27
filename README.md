# [Dialtone](https://dialtone.earth)

Dialtone is aspirationally a **robotic video operations network** designed to allow humans and AI to cooperatively train and operate thousands of robots simultaneously with low latency. Dialtone is open source and free to use and lets people get paid to build, train and operate robots.

![dialtone](./dialtone.jpg)

# Vision
Dialtone aims to combine human intuition and machine precision into a unified mesh network, making robotic hardware for factory, field, civic, and home automation widely available.
1. Humans can remotely oversee, teleoperate, and teach robots from anywhere in the world.
2. AI agents can learn from human demonstrations, process complex sensory data, and execute tasks autonomously.
3. Field Radio Uplinks (FRU) relay real-time video and telemetry through open-source radio and compute hardware.
4. A Single Software Binary (SSB) is simple to deploy and use.
5. Open Assembly Instructions allow the robot system to be assembled in a garage with the correct tools and parts.
6. Maintainable Parts and Code create a cyclic ecosystem.

## Skills
Skill are systems of systems that combine into valuable real world actions like navigating a robot or modifying and testing code.
1. Autocode: Fast, safe code evolution that scales AI-assisted control across large robot networks.
2. CLI: A single operational interface that standardizes control across distributed fleets.
3. AI: Vision and language assistance that turns operator intent into network-wide actions.
4. VPN: Private, identity-aware networking that keeps large robot networks connected and secure.
5. RSI: Collaborative planning that aligns humans and AI across multi-robot missions.
6. Marketplace: Parts and services access that scales buildout and support for large networks.
7. Bus: Scalable command and telemetry flow that coordinates many robots at once.
8. Radio: Field uplinks that maintain network control when traditional links fail.
9. Autoconfig: Automated device discovery that accelerates bringing new nodes into the network.
10. Geo: Geospatial context that enables coordinated operations over large areas.
11. CAD: Simulation-first validation that reduces risk when rolling updates across fleets.
12. Web: Public and operator visibility that extends network control beyond the CLI.
13. Social: Shared moments and coordination channels that strengthen network engagement.
14. Cyber Defense: Security automation that protects large robotic operation networks.
15. Maintenance: Cyclic parts and repair supply chains that keep distributed fleets sustainable.
16. Mock Mode: Hardware-free simulation that keeps network-scale testing and iteration moving.

# Binary Architecture: Production vs. Development
Both systems contain the same core code but differ in their capabilities.
1. `dialtone` a single binary with embeded vpn that networks robots together.
2. `dialtone-dev` a single binary with embeded vpn that networks robots together.

# Test-Driven Development (TDD)
1. A test is created for every development task.
2. The test is written before the code therefor driving the design and implementation.

# `dialtone` development and CLI
1. Use only these two tools as much as possible `dialtone.sh` CLI and `git`
2. Always run `./dialtone.sh ticket start <ticket-name>` before making any changes.
3. `dialtone.sh` is a simple wrapper around `src/dev.go`

## Clone
```bash
git clone https://github.com/timcash/dialtone.git # Clone the repo
cd dialtone
```

## Installation & Setup
```bash
git pull origin main # update main so you can integrate it into your ticket
mv -n .env.example .env # Only if .env does not exists
./dialtone.sh install # Install dev dependencies
./dialtone.sh install --remote # Install dev dependencies on remote robot
```

## Ticket Lifecycle
```bash
./dialtone.sh ticket add <ticket-name> # Add a ticket.md to tickets/<ticket-name>/
./dialtone.sh ticket start <ticket-name> # Creates branch and draft pull-request
./dialtone.sh ticket subtask list <ticket-name> # List all subtasks in tickets/<ticket-name>/ticket.md
./dialtone.sh ticket subtask next <ticket-name> # prints the next todo or process subtask for this ticket
./dialtone.sh ticket subtask test <ticket-name> <subtask-name> # Runs the subtask test
./dialtone.sh ticket subtask done <ticket-name> <subtask-name> # mark a subtask as done
./dialtone.sh ticket done <ticket-name>  # Final verification and pull-request submission
```

## Running Tests: Tests are the most important concept in `dialtone`
```bash
./dialtone.sh test ticket <ticket-name> # Run all subtask tests for the specific ticket
./dialtone.sh test ticket <ticket-name> --subtask <subtask-name> # Run a specific subtask test
./dialtone.sh test plugin <plugin-name> # Run tests for a specific plugin
./dialtone.sh test tags [tag1 tag2 ...] # Run tests matching any of the specified tags
./dialtone.sh test --list               # List tests that would run
./dialtone.sh test                      # Run all tests
```

## Logs
```bash
./dialtone.sh logs # Tail and stream local logs
./dialtone.sh logs --remote # Tail and stream remote logs
./dialtone.sh logs --lines 10 # get the last 10 lines of local logs
./dialtone.sh logs --remote --lines 10 # get the last 10 lines of remote logs
```

## Plugin Lifecycle
```bash
./dialtone.sh plugin add <plugin-name> # Add a README.md to src/plugins/<plugin-name>/README.md
./dialtone.sh plugin install <plugin-name> # Install dependencies
./dialtone.sh plugin build <plugin-name> # Build the plugin
./dialtone.sh test plugin <plugin-name> # Runs tests in src/plugins/<plugin-name>/test/
```

## Build & Deploy
```bash
./dialtone.sh build --full  # Build Web UI + local CLI + robot binary
./dialtone.sh deploy        # Push to remote robot
./dialtone.sh diagnostic    # Run tests on remote robot
./dialtone.sh logs --remote # Stream remote logs
```

## GitHub & Pull Requests
```bash
./dialtone.sh github pr           # Create or update a pull request
./dialtone.sh github pr --draft   # Create as a draft
./dialtone.sh github check-deploy # Verify Vercel deployment status
```

## Git Workflow
```bash
git status                        # Check git status
git add .                         # Add all changes
git commit -m "feat|fix|chore|docs: description" # Commit changes
git push --set-upstream origin <branch-name> # push branch to remote first time
git push                          # Push updated branch to remote
git pull origin main              # Pull changes
git merge main                    # Merge main into current branch
```

## Develop the WWW site
```bash
./dialtone.sh www dev # Start local development server
./dialtone.sh www build # Build the project locally
./dialtone.sh www publish # Deploy the webpage to Vercel
./dialtone.sh www logs <deployment-url-or-id> # View deployment logs
./dialtone.sh www domain [deployment-url] # Manage the dialtone.earth domain alias
./dialtone.sh www login # Login to Vercel
```

# Development Hierarchy
1. **Ticket**: The first step of any change. Ideal for adding new code that can patch `core` or `plugin` code without changing it directly.
2. **Plugin**: The second step is integrating new code into specific feature areas.
3. **Core**: Core code is reserved for features dealing with networking and deployment (dialtone/dialtone-dev). It is the minimal code required to bootstrap the system.


## Architecture Overview
Dialtone is built on a "Network-First" architecture, prioritizing secure, low-latency communication between distributed components.

```mermaid
---
config:
  layout: elk
  look: classic
  theme: dark
---
flowchart TD
    AI[AI Inference Workers]
    Browser[Web Dashboard / RSI]
    Bus[NATS Message Bus]
    VPN[Tailscale Mesh VPN]
    Web[Web Dashboard / RSI]
    CLI[Control CLI]
    Cam[Camera/V4L2]
    Controller[Controller]
    Robot_Radio[Radio]
    Field_Uplink[Field Uplink]
    subgraph Operator
        Browser
    end
    subgraph "Dialtone"
        direction LR
        Bus
        VPN
        Web
        CLI
    end
    subgraph Raspi
        Dialtone
    end
    subgraph Robot
        Dialtone
        Raspi
        Cam
        Controller
        Robot_Radio
    end
    subgraph Cloud
        AI
    end
    Robot_Radio --> Field_Uplink
    Field_Uplink --> Cloud
    Field_Uplink --> Operator
    Cam --> Raspi
    Controller --> Raspi
```

## Project Structure
```
dialtone/
├── tickets/           # All tickets
├── src/               # All source code
│   └── plugins/       # All plugins
├── test/              # Core test files
├── docs/              # VM and container docs
│   └── vendor/<vendor_name>/  # Vendor docs
├── example_code/      # Integration/design examples
├── dialtone.sh        # CLI wrapper for `src/dev.go` (Linux/macOS/WSL)
└── README.md          # Repo overview
```

## Ticket Structure
For tickets created via `./dialtone.sh ticket start <ticket-name>`:
```
tickets/<ticket-name>/
├── ticket.md          # Requirement doc (from template)
├── task.md            # Scratchpad for tracking progress
├── code/              # Local code playground
└── test/              # Ticket-specific verification tests
```

## Plugin Development Structure
For new plugins created via `./dialtone.sh plugin create <plugin-name>`:
```
src/plugins/<name>/
├── app/               # Application code
├── cli/               # CLI command code
├── test/              # Plugin-specific tests
└── README.md          # Plugin documentation
```

# Data Objects
1. `ISSUE`: The source-of-truth problem statement in GitHub that drives triage and labeling.
2. `TICKET`: The local, time-boxed unit of work (about 60 minutes) that turns an ISSUE into executable subtasks.
3. `SUBTASK`: A small, ~10 minute step with a single test to keep work atomic and verifiable.
4. `TEST`: The automated check that proves a subtask works and keeps agents grounded.
5. `PLUGIN`: A modular feature area with its own CLI commands, docs, and tests.
6. `WORKFLOW`: A documented CLI-driven process that keeps long-running agent work consistent.
7. `LOG`: The primary debugging stream for local or remote diagnostics.
8. `USER`: An identity record (public key) used for auth, authorization, and preferences.
9. `SKILL`: A bundle of plugins and workflows surfaced as a single CLI command.

## ISSUE: The GitHub source-of-truth for a problem
1. ID: The GitHub issue ID.
2. TITLE: The GitHub issue title.
3. DESCRIPTION: The GitHub issue description.
4. LABELS: Priority, type, and readiness flags used by the `github` plugin.

## TICKET: The local 60-minute work unit
1. BRANCH: Git branch created or switched to for the ticket.
2. DIRECTORY: `tickets/<ticket-name>/` scaffolded with `ticket.md`, `task.md`, `code/`, and `test/`.
3. SUBTASKS: A list of 10-minute steps that each have a test and status.
4. LIFECYCLE: `ticket start` -> `subtask` loop -> `ticket done` to ready the PR.

## SUBTASK: Small, test-first unit of work
1. NAME: Kebab-case identifier used by CLI commands.
2. DESCRIPTION: Single focused change with file or behavior context.
3. TEST: One command that must fail first and pass after implementation.
4. STATUS: `todo`, `progress`, `done`, or `failed`.

## TEST: Proof that a subtask is complete
1. SCOPE: One subtask or plugin goal.
2. COMMAND: A `dialtone.sh test ...` invocation tied to the subtask.
3. OUTCOME: Must fail before the change and pass after.

## PLUGIN: Modular feature area with its own tooling
1. README: High-level plugin intent and command reference.
2. CLI: Commands exposed through `dialtone.sh`.
3. TESTS: Plugin-specific tests under `src/plugins/<name>/test/`.
4. LIFECYCLE: `plugin add` -> `install`/`build` -> `test`.

## WORKFLOW: Agent grounding for long-running tasks
1. DOC: A guide in `docs/workflows/` defining how to operate.
2. CLI: References the commands and expectations for the flow.
3. PURPOSE: Keeps planning, execution, and verification consistent.

## LOG: Primary debugging stream
1. LOCAL: `./dialtone.sh logs` for local debugging.
2. REMOTE: `./dialtone.sh logs --remote` for robot diagnostics.
3. CONTEXT: Use for tracing failures during tests or deploys.

## USER: Identity record for access and preferences
1. PUBLIC KEY: The primary identifier for a user.
2. AUTH: Used for authentication and authorization decisions.
3. PREFS: Stores user preferences for agent behavior.

## SKILL: Bundled capability surface
1. WRAPS: A collection of plugins and workflows.
2. CLI: Exposed as a single agent-facing command.
3. GOAL: Makes repeatable agent behavior easy to invoke.

## WORKFLOW: Agent-focused CLI process
1. SCOPE: A long-running task category (issue review, ticket, subtask expansion).
2. SOURCE: Documented in `docs/workflows/` with step-by-step guidance.
3. OUTCOME: Clear checks and artifacts that keep agents aligned.

# Workflows
1. [Issue Review](docs/workflows/issue_review.md)


# Join the Mission
Dialtone is an open project with an ambitious goal. We are looking for:
- **Robot Builders**: To integrate their hardware and test the system.
- **AI Researchers**: To deploy models into the RSI and automate tasks.
- **Developers**: To help us build the most accessible robotic network on Earth.