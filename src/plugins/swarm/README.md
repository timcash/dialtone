# Swarm Plugin: V2 Distributed Data

The Swarm Plugin provides a decentralized, multi-writer data layer for Dialtone using the Holepunch ecosystem (**Autobase v2**, **Hypercore**, **Hyperswarm**).

## 🚀 Quick Start

### 1. Install Dependencies
```bash
./dialtone.sh swarm install
```

### 2. Start a Warm Peer
A "Warm Peer" stays online to hold DHT topics open, speeding up discovery and providing a stable bootstrap for new nodes.
```bash
# Starts a warm peer on the 'dialtone-v2' topic in the background
cd src/plugins/swarm/app
pear run warm.js dialtone-v2
```

### 3. Run a Client Test
Verify you can connect, authorize, and sync data.
```bash
cd src/plugins/swarm/app
pear run test_warm_connect.js
```

## 🏗 Core Architecture: The V2 Way

### 1. Dual-Topic Handshake
Nodes join two separate topics:
- **`:bootstrap` (Plain-text)**: Used for discovery and exchanging writer keys. Peers announce their `WRITER_KEY` and `BASE_KEY` here.
- **`:data` (Encrypted)**: Used for actual Hypercore replication once authorization is established.

### 2. Autobase v2 (Apply Pattern)
We use the **Event Sourcing** pattern. Instead of writing directly to a database, you append "Ops" to a log. The `apply` function processes these ops to build a deterministic view (e.g., a Hyperbee K/V store).

**CRITICAL RULES**:
- **Async Apply**: The `apply` function MUST be `async` and `await host.addWriter`.
- **Convergence (Acking)**: Use `ackInterval` or manual `base.ack()` to ensure the linearizer advances the "Signed Length."
- **Authorization**: A follower only becomes `writable` after an existing writer adds it via `host.addWriter` in their `apply` logic.

## 🛠 CLI Reference

| Command | Description |
| :--- | :--- |
| `./dialtone.sh swarm status` | Show live peer counts and connection info. |
| `./dialtone.sh swarm test` | Run the full V2 integration suite. |
| `pear run warm.js <topic>` | Maintain a persistent presence on the DHT. |

## 🧪 Testing Levels
We use incremental levels to isolate issues:
- **Level 1**: Basic P2P Replication (Hypercore + Swarm).
- **Level 2**: Static Authorization (Manual `addWriter` verification).
- **Level 3**: Automated Handshake (Topic-based key exchange).
- **Level 4**: Full V2 Lifecycle (Production `AutoLog`/`AutoKV` classes).

---
*Note: Pear apps use the Bare runtime. Use `bare-fs`, `bare-path`, and `bare-os` for maximum performance.*