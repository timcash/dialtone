# Swarm Plugin Help

Usage: `./dialtone.sh swarm [COMMAND] [ARGS]`

## Commands

| Command | Description |
| :--- | :--- |
| `install` | Install dependencies (npm + bun) |
| `dashboard` | Launch web dashboard (http://127.0.0.1:4000) |
| `start <topic> [name]` | Start a background node for a topic |
| `stop <pid>` | Stop a background node by PID |
| `join <topic>` | Join a topic in foreground |
| `list` | List all managed nodes |
| `status` | Show live peer counts and latency |
| `test` | Run integration tests (Pear) |
| `test-e2e` | Run E2E tests (Puppeteer) |
| `dev <topic>` | Run node with devtools |

## Examples

```bash
# Setup
./dialtone.sh swarm install

# Start the UI
./dialtone.sh swarm dashboard

# Join the 'errors' topic
./dialtone.sh swarm start errors "error-logger"

# Check what's running
./dialtone.sh swarm list
./dialtone.sh swarm status

# Stop a node
./dialtone.sh swarm stop 1234
```
