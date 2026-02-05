# DIALTONE v2 (Virtual Librarian)

Dialtone v2 centers on a task DAG shared over Hyperswarm. Tasks are the composable unit of work, streamed by topic, signed on completion, and negotiated collaboratively between `DIALTONE:`, `LLM:`, and `USER:`.

## Goals
- Replace ticket/subtask workflows with a task-first DAG.
- Enable async, peer-to-peer task streams by topic.
- Make task state auditable via signatures and logs.
- Keep the interface simple: a log stream with context.

## System Views
The system can be viewed through three distinct lenses:
1. **The Swarm View**: Connectivity. Handles peer discovery, latency, and raw message passing.
2. **The Dialtone View** (Virtual Librarian): Intelligence. Turns messages into a `task-graph`, manages the DAG, and enforces signatures.
3. **The Autobase View**: Consistency. Ensures that regardless of who writes to a topic, all peers see the same causal history.

## Topic Layout
Dialtone organizes swarm traffic into 4 primary topic types:

1. **index**: The public entry point.
   - Capability tags (what this peer can do).
   - Latency checks with other nodes.
   - Discovery of other topics.

2. **errors**: System-wide error reporting.
   - Simple metadata.
   - Links to details (e.g. log streams or task IDs).

3. **sessions**: Multi-agent working channels.
   - Where the `task-graph` lives.
   - Collaborative work happens here.
   - Archived for analysis/training when done.

4. **k/v**: Shared Key/Value Store.
   - Powered by **Hyperbee** + **Autobase**.
   - Distributed configuration and state.

## Key/Value Store
The `k/v` topic is a shared state store built using `hyperbee` and `autobase` (potentially via `autobee`).
- **Structure**: Sparse B-tree implementation.
- **Usage**: Storing Swarm-wide configuration, persistent state, or shared context that outlives specific sessions.
- **Mechanics**: Peers append operations to their local input log; Autobase linearizes these into a global order that feeds the Hyperbee index.

## Task Model
Each task is a node in a DAG, typically living in a `sessions` topic.

Required fields:
- `id`: stable task identifier
- `title`: short descriptive name
- `topic`: Hyperswarm topic name
- `dependencies`: list of task ids
- `budget`: numeric budget (time, cost, or combined)
- `score`: current priority or quality score
- `success_probability`: 0.0 to 1.0
- `signatures_required`: count or list of required signers
- `status`: `open` | `claimed` | `blocked` | `done`
- `tags`: optional labels (e.g. `needs-review`)

Rules:
- A task is `done` when all dependencies are `done` and required signatures are present.
- Any peer can propose a dependency or tag in the stream.
- Tasks can require multiple signatures to complete.

## CLI (new plugin name: `task`)
The new plugin name is `task`. Every completion is signed with:

`./dialtone.sh task --sign <task-id>`

Suggested CLI primitives:
- `./dialtone.sh task add <task-id> --title "..." --topic <topic>`
- `./dialtone.sh task dep add <task-id> <depends-on-id>`
- `./dialtone.sh task claim <task-id>`
- `./dialtone.sh task --sign <task-id>`
- `./dialtone.sh task graph --topic <topic>`

## Definition of Done
Dialtone v2 is a distributed, collaborative task graph. The log stream is the interface, the DAG is the source of truth, and signatures are the completion contract.