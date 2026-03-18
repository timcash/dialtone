# Development Runtime Plan

This replaces the older ad hoc REPL planning docs.

## Goal

Make local development and LLM-driven operation predictable when using:

- `./dialtone.sh ...`
- `repl src_v3`
- `chrome src_v3`
- long-lived local services like CAD, Chrome, and Cloudflare

The desired operator model is:

1. `./dialtone.sh <command>` always routes through a healthy local REPL leader.
2. The REPL leader stays running in the background and restarts cleanly.
3. `DIALTONE>` stays short and high-signal.
4. Detailed logs stay in the subtone room or service log files.
5. Browser-driven plugins can reconnect without forcing full browser restarts.

## Current Pain Points

- REPL autostart still uses detached `go run` in `src/plugins/repl/src_v3/go/repl/leader_ensure_v3.go`.
- REPL health is still mostly "NATS port is reachable", not "leader command loop is healthy".
- Chrome v3 daemon still owns too much lifecycle: browser process, allocator connection, managed tab, and command bus.
- Chrome `reset` still behaves closer to "restart browser state" than "reset managed session only".
- Chrome uses shared default ports in `src/plugins/chrome/src_v3/types.go`, which is fragile for multi-role and iterative workflows.
- Long-lived foreground commands still make iterative REPL work awkward.

## Work Order

1. Harden REPL leader startup and health.
2. Add REPL leader state and doctor/status inspection.
3. Split Chrome daemon responsibilities so reconnect does not imply browser restart.
4. Make Chrome session reset tab-scoped by default.
5. Move Chrome role/runtime state into explicit files, not only in-memory fields.
6. Improve service lifecycle UX for plugins that run for a long time.

## Deliverables

- A stable REPL leader background lifecycle.
- A real local state file for the REPL leader.
- A reconnectable Chrome daemon that can preserve a live Chrome instance.
- Better defaults for LLM-driven workflows.
- Updated READMEs once the runtime behavior is stable.

## Success Criteria

- Killing the shell does not lose the REPL leader.
- `./dialtone.sh <command>` does not need `--inject` or `--nats-url`.
- REPL status can explain whether the leader is healthy, stale, or partially started.
- Chrome status can explain whether the daemon is healthy, whether Chrome is alive, and whether the managed session is reusable.
- CAD/UI tests can reuse Chrome infrastructure without stale console state or browser restarts between ordinary runs.

