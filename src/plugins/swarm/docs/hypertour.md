# Hyper Ecosystem Outline (Raven Scott)

Source: https://blog.raven-scott.fyi/the-bulding-blocks-of-peer-to-peer

## Overview
- Purpose: survey of Hyper ecosystem building blocks for P2P apps
- Scope: peer discovery, replication, multi‑writer logs, key/value stores, file sharing

## HyperSwarm (Peer Discovery)
- Topics are 32‑byte hashes of human‑readable strings
- Server vs client discovery modes
- Lifecycle controls: join/flush/refresh/destroy
- Direct connections via known peer keys

## Hyper‑DHT (Networking Backbone)
- Public‑key‑based identification and routing
- P2P servers and direct connections
- Lookup/announce APIs
- Mutable/immutable records for discovery data

## Autobase (Multi‑Writer Log Merge)
- Merges multiple Hypercore inputs into a deterministic log
- Produces a linearized view for higher‑level indexing
- Apply/open hooks to build custom views
- Reordering/causal updates handled via replay

## Hyperdrive (Distributed Filesystem)
- Uses Hyperbee for metadata, Hypercore for file contents
- Familiar filesystem operations (put/get/del/list)
- Replication via Hyperswarm
- Local import/export workflows

## Hyperbee (Key/Value B‑Tree)
- Append‑only key/value store over Hypercore
- Encoding options for keys/values
- Batch writes, history, and diff streams
- Snapshot‑like behavior for consistent reads

## Hypercore (Append‑Only Log)
- Immutable log with signed blocks and Merkle trees
- Replication over arbitrary transports
- Sessions and snapshots for read consistency
- Truncation/forking and block clearing

## HyperDB (High‑Level DB Layer)
- Schema‑driven collections and indexes
- Backends: Hyperbee (P2P) or RocksDB (local)
- Query APIs with range filters and streaming helpers
- Transactions, snapshots, and flush semantics

## Integration Patterns
- Collaborative apps: Autobase + Hyperbee + Hyperswarm
- Filesystems: Hyperdrive + Hyperswarm
- P2P logs: Hypercore + Hyperswarm

## Takeaways
- Hyper ecosystem is modular but composable
- Autobase handles multi‑writer ordering; Hyperbee provides indexed views
- Hyperswarm/DHT handle discovery and transport
