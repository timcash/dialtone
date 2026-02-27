# NATS Leaf Node Architecture for Dialtone REPL

## Overview
Modify the `repl` plugin architecture to ensure local REPL sessions always work even when disconnected from the leader. This is achieved by:
1.  Running an **embedded NATS server** on every node (client or leader).
2.  Configuring client NATS servers as **leaf nodes** that connect to the leader's NATS server.
3.  Ensuring local clients always connect to their local embedded NATS first.

## Requirements
- Local REPL must be functional without a leader.
- When a leader is available, local traffic (chat, commands) should replicate to the leader's "hub".
- Architecture should be resilient to leader downtime.

## Design Changes

### 1. NATS Configuration
- **Leader (Hub)**:
    - Standard NATS server.
    - Listens for leaf node connections (typically on port 7422).
- **Client (Leaf)**:
    - Embedded NATS server.
    - Configured with a `leafnodes` block pointing to the Leader's NATS URL.
    - Local clients connect to `nats://127.0.0.1:4222`.

### 2. Plugin Code Updates (`src/plugins/repl/src_v1/go/repl/repl.go`)
- Update `RunJoin` and `RunLeader` to handle the new NATS topology.
- Refactor `connectNATS` to support leaf node configuration.
- Update `logs.StartEmbeddedNATSOnURL` (or add a new function) to accept leaf node configuration options.

### 3. CLI Options
- Add `--leaf-remotes` to `join` and `leader` commands to specify hub locations.

## Implementation Plan

### Phase 1: NATS Server Enhancement
- Modify `src/plugins/logs/src_v1/go/nats.go` to support leaf node options in `nserver.Options`.
- Add `StartEmbeddedLeafNATS(localURL string, remotes []string)` to `logs` plugin.

### Phase 2: REPL Plugin Logic
- Update `RunJoin` to always start an embedded NATS server if not already running.
- Configure the embedded NATS as a leaf node if a leader URL is provided.
- Ensure the local REPL client connects to `localhost`.

### Phase 3: Leader Enhancements
- Update `RunLeader` to optionally listen for leaf node connections.

### Phase 4: Verification
- Test local REPL with no leader.
- Test leader-follower synchronization via leaf node bridging.
- Test leader disconnect/reconnect behavior.

## Implementation Steps

1.  **Modify `logs` plugin**:
    - Add `LeafNodeOptions` to `StartEmbeddedNATSOnURL`.
2.  **Modify `repl` plugin**:
    - Update `RunJoin` to initialize local NATS.
    - Update `RunLeader` to allow leaf connections.
3.  **Tests**:
    - Add a test case for leaf node connectivity in `src/plugins/repl/src_v1/test/99_multiplayer/suite.go`.
