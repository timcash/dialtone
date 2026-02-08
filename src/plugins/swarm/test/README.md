# Swarm Plugin: Testing Framework

The Swarm Plugin uses a multi-layered testing strategy to verify decentralized consensus and P2P synchronization.

## 🏗 Test Structure

We categorize tests into three distinct layers:

### 1. Incremental Protocol Levels (`app/test_level*.js`)
These tests are used to isolate specific failures in the Holepunch stack. They should be run in sequence when debugging synchronization issues.

- **Level 1 (Corestore)**: Verifies basic P2P connectivity and raw Hypercore replication.
- **Level 2 (Static Autobase)**: Verifies manual writer authorization and view linearizing.
- **Level 3 (Handshake)**: Verifies automated key exchange over the `:bootstrap` topic.
- **Level 4 (V2 Classes)**: Verifies the high-level `AutoLog` and `AutoKV` classes.

### 2. Convergence Suites (`app/test_v2.js`)
Comprehensive tests that simulate multiple nodes performing concurrent operations.
- **Goal**: Ensure all nodes reach an identical state (hash convergence) under load.
- **Features**: Periodic acks, random write patterns, and automated convergence reporting to `TEST.md`.

### 3. Environmental / "Warm" Tests (`app/test_warm_connect.js`)
Tests that verify connectivity against long-lived infrastructure.
- **Goal**: Verify that new nodes can join an existing "Warm Peer" and sync history correctly.
- **Usage**: Requires a `warm.js` process running in the background.

## 🚀 Execution Guide

### Running All Levels
```bash
cd src/plugins/swarm/app
pear run test_level1_corestore.js
pear run test_level2_autobase_static.js
pear run test_level3_handshake.js
pear run test_level4_full_v2.js
```

### Running the Convergence Test
```bash
cd src/plugins/swarm/app
pear run test_v2.js lifecycle
```

### Infrastructure-based Test (Simulation of Real Peer)
1. **Start the anchor**: `pear run warm.js dialtone-v2`
2. **Run the client**: `pear run test_warm_connect.js`

## 🛠 Best Practices for Agents
- **Isolate Storage**: Always use `path.join(os.tmpdir(), ...)` with a unique timestamp for every test run to avoid Corestore locking errors.
- **Wait for Flush**: Always `await discovery.flushed()` before assuming peers can find each other.
- **Monitor Acks**: If data isn't appearing in the view, verify that `base.ack()` is being called (either via `ackInterval` or manually).
- **Check MDNS**: When testing locally on one machine, ensure `{ mdns: true }` is passed to the `Hyperswarm` constructor.
