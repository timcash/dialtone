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
