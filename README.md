# [Dialtone](https://dialtone.earth)
![dialtone](./dialtone.jpg)
- Open civic communications infrastructure
- Virtual librarian for mathematics, physics, and engineering
- Publicly owned and assembled robots, radios and tools
- A low latency geospatial tasking and intelligence system
- A marketplace for parts, services, training, and integrations
- Tools for creating quantum resistant encrypted keys and signatures

### Example use cases
- Small goods delivery and distribution
- Localized manufacturing and industrial automation
- Forest Fire Detection and Suppression
- Disaster Response and Recovery
- City Beautification and Maintenance
- Civic Gardens and Urban Agriculture
- Cloud Seeding to enhance cold season snowfall
- Infrastructure Inspection and Maintenance
- Environmental Monitoring and Conservation
- Public Safety and Emergency Response
- Search and Rescue

### `DIALTONE` a Virtual Librarian 
  - guides individuals and teams to actionable tasks
  - finds civic problems and turns them into questions
  - turns questions into tickets
  - turns tickets into tests
  - turns tests into upgrades across real machines

### Build kits: hardware you can assemble
- Radio kit: an open, garage-buildable field uplink for moving control, telemetry, and video when standard networks are unreliable.
- Robot kit: a reference robot stack (compute, sensors, control) designed to join the network, publish telemetry, and accept upgrades safely.

### Marketplace
- Dialtone links builders and operators to parts, services, and integrations that work with the platform.
- Hardware: bill of materials, sourcing links, and compatible alternates for radio and robot kits.
- Services: assembly help, repairs, calibration, and field support.
- Software: plugins, integrations, and deployment recipes that can be installed and tested through the same ticket-driven workflow.
- Trust signals: test coverage, compatibility notes, and operational constraints (power, bandwidth, range, compute).

### Deep technical education
- Structured learning paths for the math and engineering behind real systems:
  - networking (identity, encryption, routing, failure modes)
  - telemetry (schemas, time-series, logging, observability)
  - controls (state estimation, feedback, tuning)
  - mapping and geospatial systems (frames, projections, uncertainty)
  - radio links (link budgets, latency, throughput, antennas)
- Learning is tied to doing: each concept maps to tickets, tests, and small deployable changes.

### Physical shops and labs
- Find a local shop or lab to build and test radio and robot kits.
- Access shared equipment (soldering, RF test gear, 3D printing, calibration tools) and repeatable build procedures.
- Run acceptance tests on hardware before field deployment (power, thermal, radio link, sensor and control checks).

### What Dialtone provides
- Private encrypted networking for robots and operators (identity-aware connectivity).
- A message bus for commands, telemetry, events, logs, and streams.
- A CLI-first workflow for repeatable development, validation, and deployment.
- An Earth Library: geospatial context for fleet state (location, environment, and operational context).

### Quickstart

```bash
git clone https://github.com/timcash/dialtone.git
cd dialtone
./dialtone.sh install
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket next
```

### How work changes land safely (tickets)
Dialtone uses a ticket workflow to keep changes small, testable, and reviewable.

```bash
./dialtone.sh ticket add <ticket-name>         # scaffold a ticket
./dialtone.sh ticket start <ticket-name>       # branch + draft PR
./dialtone.sh ticket next                      # run the next test and advance state
./dialtone.sh ticket done                      # finalize and mark PR ready
```

### Tickets as work, budgets, and income
- A ticket is a unit of work that can span hardware, logistics, operations, and software.
- Tickets can carry budgets and can be funded as paid work by individuals, organizations, or public programs.
- A ticket is written so someone else can execute it: clear scope, acceptance criteria, and a test or verification plan.
- Work becomes reusable when tickets land as documented procedures, tested code, validated hardware builds, or repeatable supply and delivery workflows.

Example tickets:
- Sourcing a metal supply (vendors, specs, lead times, acceptance tests).
- Engineering a new part (CAD, manufacturing notes, fit checks, field validation).
- Delivery of small goods (routing, handling, tracking, proof of delivery).
- Software upgrades to vision models (dataset updates, evaluation, deployment, rollback plan).
- Improving documentation (install steps, operator runbooks, troubleshooting guides).

### Neural network management system
- A neural network management system keeps model changes testable, comparable, and safe to deploy.
- It manages datasets, training runs, evaluation suites, and deployment artifacts so upgrades can be audited and reproduced.
- It supports offline evaluation and on-robot validation before a model is promoted to field use.
- It ties model changes back to tickets so model version, metrics, and rollout decisions are recorded alongside the work.

### Vision (why the pieces exist)
- Learning loop: tests + logs + telemetry turn fleet experience into reusable building blocks.
- Remote operations: supervision and teaching are required to move from prototypes to maintained systems.
- Security by default: encrypted, identity-aware connectivity is a prerequisite for safe control and third-party integration.
- Field connectivity: radios and edge links keep control and data moving when standard networks fail.
- Geospatial ground truth: shared spatial context enables coordination over real terrain.
- Education: the library compounds only if teams can learn and apply it.

Together, these create a reliable substrate that a library (components, workflows, validated upgrades) and a marketplace (parts, services, training, integrations) can build on top of.

### Plugins (capabilities you can add)
Plugins extend Dialtone without modifying core networking and deployment primitives. Common capability areas include:
- Ops + runtime: VPN, Bus, Radio, Logs, Web dashboards.
- Development: Autocode (ticket/test loop), Mocks, CAD (simulation-first validation).
- Fleet context: Geo (spatial), Weather (environmental inputs), Autoconfig (bring-up/enrollment).
- Human + org: RSI (planning/coordination), Marketplace (distribution and integrations), Maintenance, Cyber, Social.

```bash
./dialtone.sh plugin add <plugin-name>
./dialtone.sh plugin install <plugin-name>
./dialtone.sh plugin build <plugin-name>
./dialtone.sh plugin test <plugin-name>
```

### Logs
Log lines are formatted as 
```shell
[timestamp | level | file:function:line] message
```
Examples
```shell
[2026-02-03T12:00:00.123Z07:00 | INFO  | main.go:run:42] starting dialtone
[2026-02-03T12:00:01.217Z07:00 | ERROR | vpn.go:up:133] failed to bring vpn up: permission denied
```

```bash
./dialtone.sh logs
./dialtone.sh logs --remote
./dialtone.sh logs --lines 50
```

### Build & deploy

```bash
./dialtone.sh build
./dialtone.sh deploy
./dialtone.sh diagnostic
```

### WWW development

```bash
./dialtone.sh www dev
./dialtone.sh www build
./dialtone.sh www publish
```

### Workflows and docs
- `docs/workflows/issue_review.md`
- `docs/workflows/ticket.md`
- `docs/workflows/subtask_expand.md`

