# Swarm Plugin: V2 Testing Workflow

This framework verifies the **Holepunch** P2P stack using a progressive, layered approach. Tests are numbered `test_N_...` and should generally be run in order when debugging the protocol.

## 🏗 The Layered Workflow

### Step 1: Infrastructure Layer (`test_1_warm_node.js`)
Ensures a stable DHT anchor is running.
- **Action**: Starts `warm.js` in the background if not already running.
- **Purpose**: Provides a persistent bootstrap for all subsequent tests, preventing DHT "cold start" delays.
- **Run**: `pear run test_1_warm_node.js`

### Step 2: Transport Layer (`test_2_corestore.js`)
Verifies raw data movement between two peers.
- **Action**: Node A appends to a Hypercore; Node B replicates it.
- **Purpose**: Isolates Hyperswarm discovery and Corestore replication from Autobase logic.
- **Run**: `pear run test_2_corestore.js`

### Step 3: Consensus Layer (`test_3_autobase_static.js`)
Verifies the multi-writer linearization engine.
- **Action**: Node A manually authorizes Node B's key.
- **Purpose**: Verifies `async apply` logic and `base.ack()` convergence without the complexity of handshakes.
- **Run**: `pear run test_3_autobase_static.js`

### Step 4: Protocol Layer (`test_4_handshake.js`)
Verifies automated key exchange.
- **Action**: Peers exchange keys over the `:bootstrap` topic and self-authorize.
- **Purpose**: Tests the plain-text handshake used to "warm up" the encrypted data swarm.
- **Run**: `pear run test_4_handshake.js`

### Step 5: API Layer (`test_5_full_v2.js`)
Verifies the production `AutoLog` and `AutoKV` classes.
- **Action**: Simple setup using the high-level V2 API.
- **Purpose**: Ensures the abstract classes correctly wrap the underlying protocol logic.
- **Run**: `pear run test_5_full_v2.js`

### Step 6: Convergence Suite (`test_6_convergence.js`)
Stress tests the system with concurrent writers.
- **Action**: Multiple nodes perform random writes for 30-60s.
- **Purpose**: Verifies that all nodes reach an identical state (Merkle hash match) under load.
- **Run**: `pear run test_6_convergence.js lifecycle`

### Step 7: Environment Integration (`test_7_warm_connect.js`)
Verifies connectivity against the long-lived warm node.
- **Action**: Client connects to the keys printed by Test 1.
- **Purpose**: Simulates a real-world scenario where a new user joins an existing stable swarm.
- **Run**: `pear run test_7_warm_connect.js`

---

## 🚀 Quick Execution Guide

To verify the entire stack from scratch:

```bash
cd src/plugins/swarm/app

# 1. Start/Verify Infrastructure
pear run test_1_warm_node.js

# 2. Run sequential protocol tests
pear run test_2_corestore.js
pear run test_3_autobase_static.js
pear run test_4_handshake.js
pear run test_5_full_v2.js

# 3. Perform final convergence check
pear run test_6_convergence.js lifecycle
```

## 🛠 Troubleshooting for Agents
- **Corestore Locks**: If a test hangs or throws "Access Denied", ensure no other process is using the same `storage` directory.
- **Signed Length**: In Autobase v2, views do not advance until a quorum of indexers acks. Check `ackInterval` if the view length remains `0`.
- **MDNS**: Local testing **requires** `{ mdns: true }` in the Hyperswarm config to bypass DHT discovery for peers on the same machine.