# Swarm Plugin

The Swarm plugin enables peer-to-peer connectivity for Dialtone using Hyperswarm and the Pear runtime.

## CLI Usage

### Lifecycle Commands
- `./dialtone.sh swarm install`: Install plugin dependencies (npm).
- `./dialtone.sh swarm test`: Run Go-based multi-peer integration tests.
- `./dialtone.sh swarm test-e2e`: Run consolidated Node+Puppeteer E2E tests.

### Node Management
- `./dialtone.sh swarm <topic>`: Join a swarm topic in the foreground.
- `./dialtone.sh swarm start <topic>`: Start a background node.
- `./dialtone.sh swarm stop <pid>`: Stop a background node by PID.
- `./dialtone.sh swarm list`: List all running swarm nodes managed by Dialtone.
- `./dialtone.sh swarm status`: Live "top-like" report showing peer counts and average latency.

### Web Dashboard
- `./dialtone.sh swarm dashboard`: Launch the web-based swarm dashboard on port 4000.

## Implementation Details

### Node Tracking
Background nodes are tracked in `~/.dialtone/swarm/nodes.json`. Each node periodically writes its state (peers, latencies) to a node-specific status file in the same directory.

### Health Monitoring
The `list` and `status` commands dynamically reconcile process IDs and check for "alive" status using Unix signals.

### E2E Testing
The consolidated E2E test suite (`src/plugins/swarm/test/swarm_orchestrator.ts`) uses Node and Puppeteer to orchestrate the entire lifecycle, including launching the dashboard and capturing browser console logs/errors.
