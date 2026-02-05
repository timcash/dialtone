# Swarm Plugin Help

Usage: `./dialtone.sh swarm [COMMAND] [ARGS]`

## Commands

| Command | Description |
| :--- | :--- |
| `install` | Install dependencies (npm + bun) |
| `dashboard` | Launch web dashboard (http://127.0.0.1:4000) |
| `start <topic> [name]` | Start a background node for a topic |
| `stop <pid>` | Stop a background node by PID |
| `list` | List all managed nodes |
| `status` | Show live peer counts and latency |
| `test` | Run integration tests (Pear) |
| `test-e2e` | Run E2E tests (Puppeteer) |

# User Guide

## 1. Starting the Swarm

 Initialize the swarm environment and start the dashboard to visualize the network.

```shell
# Install dependencies
./dialtone.sh swarm install

# Start the dashboard (runs on http://127.0.0.1:4000)
./dialtone.sh swarm dashboard

# Start a background node on the 'index' topic
./dialtone.sh swarm start index "main-node"

# Check status of running nodes
./dialtone.sh swarm status
```

## 2. Adding a Task (Design)
Tasks are the unit of work in Dialtone v2. They are broadcast over topics.

```shell
# Add a new task to the 'sessions' topic
# Note: This uses the 'task' plugin alias (proposed)
./dialtone.sh task add "fix-login-bug" \
  --topic sessions \
  --title "Fix login timeout issue" \
  --tags "bug,high-priority"

# Claim a task
./dialtone.sh task claim "fix-login-bug"

# Mark as done (requires signature)
./dialtone.sh task --sign "fix-login-bug"
```

## 3. K/V Store (Hyperbee + Autobase)
The K/V store is a distributed, multi-writer database where all peers converge to the same state using Autobase (causal ordering) and Hyperbee (B-tree index).

### How it works
1.  **Writes**: Peers append operations to their local input log.
2.  **Sync**: Autobase linearizes all input logs into a global order.
3.  **Index**: Hyperbee consumes the linearized view to update the B-tree.
4.  **Result**: Eventual consistency where all peers see the same data.

### Running the K/V Demo
We have a demonstration using ephemeral storage (fast, memory-like) to show concurrent convergence.

```shell
# Run the K/V simulation test
# This starts 2 peers, performs concurrent writes, and verifies convergence.
bun run src/plugins/swarm/test/kv.ts
```

## 4. Running Tests
Validate the swarm infrastructure using Pear (p2p) or Puppeteer (e2e).

```shell
# Run Pear-based p2p integration tests
./dialtone.sh swarm test

# Run full E2E dashboard tests (Puppeteer + Bun)
./dialtone.sh swarm test-e2e
```
