# Research Notes: Autobase v2 & Writer Authorization
- **Acknowledgment**: In Autobase v2, writers must be acknowledged to be optimistically applied to the linearized view. This can be done via `host.ackWriter(key)` or by setting an `ackInterval`.
- **Dynamic Writability**: A follower only becomes writable once its key has been added via `host.addWriter` within the `apply` function and the local Autobase has processed that block.
- **Replication**: Ensure all relevant cores (including the bootstrap core and all writer cores) are properly replicated through the swarm.
- **Local Testing**: `mdns: true` is essential for peer discovery when running multiple instances on the same machine.

# Current Progress (Updated Feb 8, 2026)

## Completed
1.  **V2 Core Classes**: `autokv_v2.js` and `autolog_v2.js` follow the `async apply` and `ackInterval` patterns.
2.  **Sequential Test Suite**: Established 8 progressive test levels (`test_1` to `test_8`) covering infrastructure to convergence.
3.  **Infrastructure Anchor**: `test_1_warm_node.js` correctly manages a background DHT anchor.
4.  **Transport & Consensus**: `test_3` and `test_4` are stabilized with discovery flushes.

## Pending / Next Steps (The "Robustness" Phase)
1.  **Stabilize Handshake**: `test_5_handshake.js` still has intermittent timeouts. Research if `hyperswarm` needs longer flush times or if `KeySwarm` should use a more persistent retry loop for the initial `WRITER_KEY` exchange.
2.  **Explicit Sync API**: Add a `.sync()` method to `AutoKV` and `AutoLog` that returns a Promise. It should resolve only when the local Autobase has caught up to all known remote heads.
3.  **Negative Authorization Tests**: Create `test_9_unauthorized_write.js`. Verify that appends from an unauthorized writer are correctly ignored by the `apply` function and do not corrupt the view.
4.  **Chaos Testing**: Extend `test_7_convergence.js` to randomly kill and restart nodes during high-volume writes to verify Oplog recovery.
5.  **Merkle Verification**: Fully implement the `verify(remoteHash)` method using the system core's Merkle root to allow nodes to prove sync status without exchanging full views.

---

# Swarm Agent Prompt: Improving the Stack
*Use this prompt to guide the next agent in refining the P2P layer.*

> "Analyze the existing 8 tests in `app/`. Identify the source of the intermittent timeout in `test_5_handshake.js`. Implement a 'Sync Helper' in the base classes that utilizes `base.update()` and `base.ack()` to guarantee data visibility. Then, implement Test 9 (Negative Auth) to ensure security."
