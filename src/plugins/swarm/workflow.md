# Swarm Plugin Workflow

Use this workflow to update the Swarm plugin end-to-end: dependencies, runtime, UI, and tests.

## Folder Structures
Swarm plugin structure:
```shell
src/plugins/swarm/
├── app/
│   ├── dashboard.html
│   ├── dashboard.js
│   ├── index.js
│   └── package.json
├── cli/
│   └── swarm.go
├── test/
│   ├── swarm_orchestrator.ts
│   └── test.go
└── README.md
```

## Command Line Help
Core swarm commands:
```shell
./dialtone.sh swarm help
./dialtone.sh swarm install
./dialtone.sh swarm dashboard
./dialtone.sh swarm start <topic> [name]
./dialtone.sh swarm stop <pid>
```

# Workflow Example

## STEP 1. Ensure environment and deps
```shell
# DIALTONE_ENV must be set in env/.env or passed with --env
./dialtone.sh swarm install
```

## STEP 2. Run the dashboard
```shell
# Starts the HTTP dashboard at http://127.0.0.1:4000
./dialtone.sh swarm dashboard
```

## STEP 3. Start and stop nodes
```shell
# Start a node for a topic (optional name)
./dialtone.sh swarm start dialtone-demo alpha

# Stop by PID (from dashboard or list)
./dialtone.sh swarm stop <pid>
```

## STEP 4. Iterate on the UI
```shell
# Edit UI files
src/plugins/swarm/app/dashboard.html
src/plugins/swarm/app/dashboard.js

# Reload the browser to see changes
```

## STEP 5. Run integration tests
```shell
# Runs multi-peer pear test using test.js
./dialtone.sh swarm test
```

## STEP 6. Run E2E tests
```shell
# Runs dashboard and validates UI with Puppeteer
./dialtone.sh swarm test-e2e

# Screenshots go to:
# ./screenshots/*.png
```
