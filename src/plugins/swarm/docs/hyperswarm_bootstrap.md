# Hyperswarm bootstrap notes (current state)

## Symptoms seen
- When multiple nodes start in parallel with no cache, each may create its own Autobase.
- Logs show `Bootstrap discovery timed out` for KV agents before any `BASE_KEY` is received.
- `swarm.flush` / `discovery.flushed` frequently time out (2s), even though peers later connect.
- Parallel startup sometimes leads to multiple distinct Autobase keys per topic.

## What the code is doing now
- `autokv` / `autolog` try to discover a bootstrap key on `<topic>:bootstrap`.
- If none is found and `requireBootstrap` is false, they create a new Autobase.
- A local in‑process registry now shares the first created base key for peers in the same process.
- Nodes then join the main topic and (optionally) keep a bootstrap host running.
- A bootstrap host writes `BASE_KEY` + `WRITER_KEY` to any peer that connects on the bootstrap channel.

## Likely causes of the failures
- **Parallel startup race:** all peers begin discovery before any host is ready, and all time out, so they each create separate bases.
- **Short discovery windows:** 2s `discovery.flushed` / `swarm.flush` timeouts are too short for DHT announce/lookup on cold start.
- **Bootstrap handshake is best‑effort:** if the key host isn't already announcing on `<topic>:bootstrap`, peers miss the key.
- **Multiple swarms in one process:** each AutoKV/AutoLog spins its own Hyperswarm; docs recommend 1 per app.

## Best‑practice pattern (from Hyperswarm docs)
- Start host, join topic as server, wait for `discovery.flushed()`.
- Start clients, join topic as client, wait for `swarm.flush()`.
- Avoid parallelizing host and clients on a cold start.

## What this suggests we do next
- Make KV/Log tests start the host first, then clients in parallel.
- Extend bootstrap timeouts or retry discovery longer before creating a new base.
- Consider a shared Hyperswarm instance per process to reduce DHT pressure.
- Persist and reuse `base.key` (cache) so joins do not depend on DHT timing.

## Open questions
- Should we require a bootstrap key (never auto‑create) in test runs?
- Do we want a long‑lived bootstrap host process for each topic?
- Are we okay with a small local registry for tests, or do we want a DHT record?
