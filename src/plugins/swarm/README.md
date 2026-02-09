# Swarm Plugin: V2 Distributed Data

The Swarm Plugin provides a decentralized, multi-writer data layer for Dialtone using the Holepunch ecosystem (**Autobase v2**, **Hypercore**, **Hyperswarm**).

## 🚀 Quick Start

### 1. Install Dependencies
```bash
./dialtone.sh swarm install
```

### 2. Scaffold a New Project
Create a fresh project template from `src_v2`.
```bash
# Creates src5 folder with minimal V2 logic and Vite UI
./dialtone.sh swarm src --n 5
```

### 3. Run Smoke Tests
Verify the dashboard and connectivity via automated headless browser tests.
```bash
./dialtone.sh swarm smoke src5
```

## 🏗 Core Architecture: The V2 Way

### 1. Dual-Topic Handshake
Nodes join two separate topics:
- **`:bootstrap` (Plain-text)**: Used for discovery and exchanging writer keys.
- **`:data` (Encrypted)**: Used for Hypercore replication once authorized.

### 2. File Structure (src_vN)
- `/bare`: Contains library-specific wrappers for `Bare`, `Pear`, and `Hypercore`.
- `/ui`: A modern Vite + TypeScript + CSS frontend dashboard.
- `index.js`: The entry point delegating between background node and dashboard modes.

### 3. Synchronization Helpers
The `AutoLog` and `AutoKV` classes now include a `.sync()` method.
```javascript
// Ensures the local view is consistent with all known remote heads
await log.sync()
```

## 🛠 CLI Reference

| Command | Description |
| :--- | :--- |
| `./dialtone.sh swarm src --n N` | Create or validate a `srcN` project folder from the V2 template. |
| `./dialtone.sh swarm smoke <dir>` | Run automated browser verification for a specific project. |
| `./dialtone.sh swarm lint` | Run the 3-stage multi-linter (Go, Prettier, and Bun/ESLint). |
| `./dialtone.sh swarm status` | Show live peer counts and connection info. |
| `./dialtone.sh swarm test` | Run the full V2 sequential integration suite. |
| `./dialtone.sh swarm warm <topic>` | Start a persistent DHT anchor for a topic. |

## 🧪 Testing Levels
We use incremental levels to isolate issues:
- **Level 1**: Basic P2P Replication (Hypercore + Swarm).
- **Level 2**: Static Authorization (Manual `addWriter` verification).
- **Level 3**: Automated Handshake (Topic-based key exchange).
- **Level 4**: Full V2 Lifecycle (Production `AutoLog`/`AutoKV` classes).
- **Negative Auth**: Verifies that unauthorized writes are dropped (Test 9).

---
*Note: Pear apps use the Bare runtime. Use `bare-fs`, `bare-path`, and `bare-os` for maximum performance.*
