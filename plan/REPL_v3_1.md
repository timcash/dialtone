# REPL v3.1

## Goal

Add a tighter REPL and logging contract test layer before the SSH and Cloudflare integration steps. The intent is to catch routing, lifecycle, renderer, and subtone-management regressions locally and cheaply before network-dependent tests run.

## Recommended Test Additions

### 1. interactive-foreground-command-lifecycle

Run a short foreground command such as `/repl src_v3 help` and assert the full index-room lifecycle:

- `Request received. Spawning subtone ...`
- `Subtone started as pid ...`
- `Subtone room: subtone-<pid>`
- `Subtone log file: ...`
- `Subtone for ... exited with code 0.`

This is the baseline contract for normal foreground commands.

### 2. main-room-does-not-mirror-subtone-payload

Start a noisy subtone and assert:

- detailed output appears only in `repl.subtone.<pid>`
- `repl.room.index` stays lifecycle-only

This protects the main-room/subtone-room split.

### 3. interactive-background-command-lifecycle

Run a long-lived local command with trailing `&` and assert:

- index room confirms background start
- `/ps` shows it as active
- it remains alive until explicitly stopped

This establishes the contract for long-running background work.

### 4. attach-detach-roundtrip

Start a subtone, attach to it, confirm attached rendering, then detach and confirm the final exit still lands in the index room.

Assert:

- attached view renders as `DIALTONE:<pid>`
- detached main view renders as `DIALTONE>`
- lifecycle exit remains visible after detach

### 5. subtone-list-live-registry

Start two local subtones and verify `subtone-list` is registry-backed and stable.

Assert the list includes:

- pid
- state
- command
- room
- log path

This should validate the live leader registry rather than old filesystem reconstruction.

### 6. subtone-log-by-pid

Start a subtone, capture its pid from lifecycle frames, then verify `subtone-log --pid <pid>` resolves the correct log and contains expected payload lines from that subtone only.

### 7. ps-matches-registry

Start and stop subtones, then assert `/ps` matches the active set exposed by the live registry.

This is the cleanup step before removing fallback behavior from `/ps`.

### 8. nonzero-exit-lifecycle

Run a command that exits nonzero and assert:

- index room reports the nonzero exit code
- subtone room contains the detailed error output
- `subtone-list` marks it as done/inactive correctly

This protects failure-path behavior, not just happy paths.

### 9. join-rendering-contract

Assert renderer behavior directly:

- index scope renders `DIALTONE>`
- attached subtone scope renders `DIALTONE:<pid>`
- raw frame messages are prefix-free

This catches branding leakage back into logic or transport.

### 10. multiple-concurrent-subtones

Launch two background subtones concurrently and verify:

- separate pids
- separate rooms
- separate log paths
- no cross-talk between attached output streams

This protects concurrent operator workflows.

## Recommended Order

Implement in this order:

1. foreground lifecycle
2. main-room isolation
3. background lifecycle
4. attach/detach roundtrip
5. `subtone-list` / `subtone-log`
6. nonzero exit
7. concurrent subtones

This keeps the early work local, deterministic, and directly tied to the REPL/logging contract before networked integration coverage.

## Exit Criteria

Before expanding SSH and Cloudflare coverage further, the suite should prove:

- main room is lifecycle-only
- subtone detail stays in subtone rooms
- foreground and background execution both behave correctly
- attach/detach works cleanly
- `subtone-list`, `subtone-log`, and `/ps` agree on active state
- failure exits are represented correctly
- concurrent subtones do not interfere with one another
