# Swarm Plugin Workflow

Use this workflow to update the Swarm plugin end-to-end: dependencies, runtime, UI, and tests.

## Folder Structures
Swarm plugin structure:
```shell
src/plugins/swarm/
├── app/
│   ├── autokv.js
│   ├── autolog.js
│   ├── dashboard.html
│   ├── dashboard.js
│   ├── index.js
│   ├── package.json
│   ├── task-dag.js
│   ├── test.js
│   └── warm.js
├── cli/
│   └── swarm.go
├── test/
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

## STEP 5. Run tests
```shell
# Runs multi-peer pear test using test.js
./dialtone.sh swarm test
```
# Findings and P2P Patterns

During the Swarm API and test refactoring, several critical patterns were established for robust P2P networking:

## 1. Decentralized Handshake (KeySwarm)
Autobase requires an initial set of writer keys to authorize nodes. In a decentralized environment, we use a dedicated **KeySwarm** on a derived topic (e.g., `topic:bootstrap`) where nodes announce their IDs.

## 2. Topic Multiplexing
When multiple abstractions (like `AutoLog` and `AutoKV`) share a single `KeySwarm` instance, handshake messages MUST include a `TOPIC:` prefix. This allows the receiver to route the `WRITER_KEY` to the correct base instance for authorization.

## 3. Periodic Key Broadcasting
P2P connections can be transient. Relying on a single handshake at connection time is often insufficient. We now implement **periodic broadcasting** (e.g., every 5s) of the topic metadata and writer keys to all active peers on the KeySwarm to ensure eventual convergence.

## 4. Replication Swarm Isolation
While the discovery layer (KeySwarm) can be shared, the **Replication Swarm** (where `base.replicate()` happens) should generally be isolated per-base or carefully multiplexed to avoid "pipe to one destination" errors.

## 5. Storage Isolation
For realistic multi-node simulation in a single environment, each node MUST use a unique `Corestore` storage path (e.g., `storage: path.join(dir, 'log')` where `dir` is unique per node). Sharing storage causes leveldb locking errors.

## 6. DHT Discovery Performance
 Discovery on the DHT can take 10-15s for new topics. Always wait for `discovery.flushed()` and implement robust retry/looping for initial bootstrap discovery.
## 7. Shared KeySwarm Listener Conflict
When sharing a single `Hyperswarm` instance for discovery across multiple `AutoLog` or `AutoKV` instances, a critical conflict occurs:
- **Problem**: Every instance attaches its own `swarm.on('connection')` listener.
- **Symptom**: All instances attempt to perform handshakes on EVERY connection, even for topics they don't own, leading to authorization hangs or duplicate messages.
- **Solution**: The `connection` handler MUST check `info.topics` (or comparable metadata) to ensure the socket is relevant to the instance's specific bootstrap topic before proceeding.

## 8. Topic-Specific Handshake Dispatcher
To support multiple bases on one swarm core:
- Use a **Dispatcher Pattern**: Each instance attaches a `data` listener that only processes lines starting with its own `TOPIC:`.
- **Listener Management**: Always remove listeners on socket `close` to prevent memory leaks and "ghost" processing on pooled connections.
- **Atomic Handshakes**: Ensure the `BASE_KEY` and `WRITER_KEY` are exchanged in the same logical pulse to prevent partial authorization states.
## 9. Line Buffering for Handshakes
P2P streams may deliver handshake messages (`TOPIC:`, `WRITER_KEY:`) in multiple chunks. To ensure reliable parsing, always implement a **Line Buffer** in the `data` listener:
```javascript
let buffer = ''
socket.on('data', (data) => {
  buffer += data.toString()
  const lines = buffer.split('\n')
  buffer = lines.pop() // Keep the trailing fragment
  for (const line of lines) {
    // Process line...
  }
})
```

## 10. Authorization Deadlock Protection
Follower nodes cannot authorize other writers until they are themselves authorized as writers on the base. Attempting to `base.append` (e.g., to authorize a peer) while not writable will hang.
- **Rule**: Only nodes that are already authorized writers (`base.writable === true`) should attempt to broadcast or process `addWriter` ops.

## 11. Interaction Loop Lifecycle
Standardize tests on a **30s Interaction Loop** to allow sufficient time for:
1.  DHT Discovery (10-15s)
2.  Key Exchange & Authorization (2-5s)
3.  Concurrent Data Appending (10-15s)
4.  Final Convergence Verification (5-10s)
