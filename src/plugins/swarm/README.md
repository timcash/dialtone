# Swarm Plugin

The Swarm plugin enables peer-to-peer connectivity for Dialtone using Hyperswarm and the Pear runtime.

## CLI Usage

### Lifecycle Commands
- `./dialtone.sh swarm install`: Install plugin dependencies into `DIALTONE_ENV` (npm + bun).
- `./dialtone.sh swarm test`: Run Go-based multi-peer integration tests.
- `./dialtone.sh swarm test-e2e`: Run consolidated Bun+Puppeteer E2E tests.
- `./dialtone.sh swarm dev [topic|dashboard] [name]`: Run Pear dev mode with devtools (defaults to dashboard).

### Node Management
- `./dialtone.sh swarm <topic>`: Join a swarm topic in the foreground.
- `./dialtone.sh swarm start <topic> [name]`: Start a background node with an optional instance name.
- `./dialtone.sh swarm stop <pid>`: Stop a background node by PID.
- `./dialtone.sh swarm list`: List all running swarm nodes managed by Dialtone.
- `./dialtone.sh swarm status`: Live "top-like" report showing peer counts and average latency.

### Web Dashboard
- `./dialtone.sh swarm dashboard`: Launch the web-based swarm dashboard at `http://127.0.0.1:4000`.
- The dashboard can start/stop nodes via HTTP endpoints:
  - `POST /start` with `{ "topic": "...", "name": "..." }`
  - `POST /stop` with `{ "pid": "1234" }`

## Development & Testing Strategy

- **Dev loop**: Use `./dialtone.sh swarm dev dashboard` to run the HTTP dashboard with Pear devtools.
- **Network behavior**: Use `./dialtone.sh swarm dev <topic> [name]` to run a live node with devtools.
- **Integration tests**: `./dialtone.sh swarm test` runs two Pear peers via `src/plugins/swarm/app/test.js`.
- **UI tests**: `./dialtone.sh swarm test-e2e` runs the dashboard and verifies it via Puppeteer.

## Implementation Details

### Node Tracking
Background nodes are tracked in `~/.dialtone/swarm/nodes.json`. Each node periodically writes its state (peers, latencies) to a node-specific status file in the same directory.

### Health Monitoring
The `list` and `status` commands dynamically reconcile process IDs and check for "alive" status using Unix signals.

### E2E Testing
The consolidated E2E test suite (`src/plugins/swarm/test/swarm_orchestrator.ts`) uses Bun and Puppeteer to orchestrate the entire lifecycle, including launching the dashboard and capturing browser console logs/errors.
