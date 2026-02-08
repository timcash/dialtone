# Swarm Agent Guide: Distributed Systems Protocol (V2)

You are a Distributed Systems Engineer working with **Holepunch**, **Autobase v2**, and **Hyperswarm**. This guide defines the protocol and mental model for the Swarm Plugin.

## 🧠 Mental Model: No Central Server
There is no "master" database. Every node is a writer of its own append-only log. Consensus is achieved via **Autobase**, which linearizes these logs into a single deterministic view using causal references.

## 📡 The V2 Protocol Flow

### 1. Discovery (KeySwarm)
Nodes join `topic + ':bootstrap'`. This is a "Warm Topic" intended for plain-text handshakes.
- **Action**: Broadcast `TOPIC:<name>\nBASE_KEY:<hex>\nWRITER_KEY:<hex>\n`.
- **Goal**: Collect `WRITER_KEY`s from peers.

### 2. Authorization (AddWriter)
In Autobase v2, a node is **Read-Only** until an existing member authorizes its key.
- **Workflow**: 
    1. Node A (Writer) receives `WRITER_KEY` from Node B (Follower) via KeySwarm.
    2. Node A appends `{ addWriter: keyB }` to its own log.
    3. The `apply` function on all nodes sees this op and calls `await host.addWriter(keyB)`.
    4. Once the block is processed, Node B's `base.writable` becomes `true`.

### 3. Replication (DataSwarm)
Nodes join the primary `topic`.
- **Action**: `store.replicate(socket)` handles the binary exchange of Hypercore blocks.
- **Constraint**: Only authorized cores are indexed by Autobase.

## 💻 Code Reference

### AutoLog & AutoKV Classes
- `ready()`: Initializes Corestore, joins swarms, and waits for DHT flush.
- `waitWritable()`: Block until authorized by a peer. Essential for startup scripts.
- `append(data)` / `put(key, val)`: Atomic operations that trigger a `base.update()`.

### The `apply` Function (The Engine)
```javascript
async function apply (nodes, view, host) {
  for (const { value } of nodes) {
    if (value.addWriter) {
      // MUST be awaited to avoid race conditions in system state
      await host.addWriter(b4a.from(value.addWriter, 'hex'), { indexer: true })
      continue
    }
    // Update the local view (Hypercore or Hyperbee)
    await view.append(value)
  }
}
```

## 🛠 Debugging Checklist for Agents

### 1. "Auth is Stuck" (Follower not becoming writable)
- **Check**: Is the `ackInterval` set? Without acks, the view may not advance to see the `addWriter` op.
- **Check**: Is the `apply` function `async`? Synchronous `apply` failing to await `addWriter` will cause internal assertion errors.
- **Check**: Are both nodes joined to the same `:bootstrap` topic?

### 2. "Data not Syncing"
- **Check**: Run `swarm.connections.size`. If 0, check `mdns: true` for local testing.
- **Check**: Ensure `store.replicate(socket)` is called on **every** connection in **both** swarms.

### 3. "AssertionError: System changes are only allowed in apply"
- **Cause**: Attempting to call `host.addWriter` outside of the `apply` loop or losing the context in an un-awaited promise.

## 🚀 Deployment Strategy
Always start a **Warm Peer** (`warm.js`) for your topic. This acts as a DHT anchor and significantly reduces "cold start" latency for new peers connecting to the swarm.