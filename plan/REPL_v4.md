# REPL v4 Cleanup Plan

## Current State

Latest validation on 2026-03-15:

- command run: `./dialtone.sh repl src_v3 test`
- result: passed end to end

That matters. `src_v3` is working again, so `src_v4` should start from a green system, not from a speculative rewrite.

The most useful lessons from getting `src_v3` green were not about missing features. They were about where the architecture is still too indirect:

- tests failed repeatedly because they were matching rendered text instead of stable semantics
- attach/detach was brittle because the system exposes process `pid` but not a stronger command/job identity
- room history is too easy to misread because event correlation is weak
- wrapper/bootstrap chatter still leaks into places where only REPL lifecycle should exist
- too much logic is split across `dev.go`, `logs`, `test`, and `repl`, so the same boundary problem shows up in multiple packages
- one shared REPL session for the suite reduced runtime and noise materially; restarting leader/join per step was mostly harness complexity, not product value
- cleanup based on `os.Exit` plus process-name matching is fragile; one failed run was enough to leave stale leaders/joins/watchers that poisoned the next run
- startup-sensitive assertions are a minority; most REPL tests want a long-lived room/session model that mirrors how real users and agents actually work

## Critical Diagnosis

`src_v3` works, but it has too many layers pretending to be transport, protocol, renderer, and process manager at the same time.

The biggest mistake to avoid in `src_v4` is carrying forward the current text-driven control model:

- user types command
- wrapper decides a lot implicitly
- REPL turns command into process activity
- logs become UI
- tests reverse-engineer intent from text

That is why small UI changes kept breaking behavior verification.

The right center is NATS, but not “NATS plus many ad hoc text streams.” It should be:

- one command envelope
- one event envelope
- one job/subtone registry model
- one renderer layer

The `src_v3` suite now demonstrates this more clearly than before:

- one leader
- one embedded NATS
- one `llm-codex` join
- many commands through the same room

That model is simpler, faster, and more representative of multiplayer REPL use than per-step restarts.

Everything else should become an adapter around that.

## Non-Negotiable v4 Principles

1. NATS stays the control plane.
   All CLI commands, RPC-style requests, lifecycle events, and pub/sub updates go through NATS.

2. Rendered text is not protocol.
   `DIALTONE>` and `DIALTONE:<pid>` are renderer output only.

3. A subtone is a managed job, not “whatever process pid we most recently saw.”
   PID can remain metadata, but it cannot be the primary identity.

4. Tests must assert envelopes and registry state first, rendered text second.

5. `dev.go` should be a thin bootstrap + client entrypoint, not the place where REPL architecture lives.

6. Sessions should be long-lived by default.
   A user or agent should join once, issue many commands, inspect jobs, attach, detach, and keep working.

7. Cleanup must be registry-driven, not process-pattern-driven.
   Process cleanup can exist as a safety net, but it cannot be the primary state/control model.

## Recommended v4 Model

### 1. One NATS Command Envelope

Every CLI command should be normalized into a single command message published to NATS.

Suggested fields:

- `command_id`
- `session_id`
- `actor`
- `workspace`
- `target`
- `argv`
- `interactive`
- `reply_subject`
- `timestamp`

This replaces today’s mixture of direct wrapper execution, REPL injection rules, and room-specific command handling.

### 2. One NATS Event Envelope

Every service should emit one canonical event type.

Suggested fields:

- `event_id`
- `command_id`
- `job_id`
- `scope`
- `kind`
- `actor`
- `room`
- `payload`
- `exit_code`
- `log_path`
- `timestamp`

Important point:
- `message` should be payload data, not preformatted UI text

### 3. Replace “subtone pid” with “job id”

`pid` is unstable and local. It is fine as metadata, but it is a bad control identity.

Use:

- `job_id`: stable identity for attach/list/kill/logs
- `pid`: optional runtime detail

This would have prevented the `src_v3` attach test race entirely. The test should not have needed to guess the “latest pid” from room history.

### 4. Create One Subtone Registry Service

Today state is spread across:

- runtime memory
- room traffic
- log files
- process cleanup code
- `subtone-list` reconstruction

That is too much.

`src_v4` should have one registry service responsible for:

- create job
- mark running
- record pid
- record room subject
- record log path
- record attachable stream subject
- record exit code
- mark done/failed/killed

`/ps`, `subtone-list`, `subtone-log`, attach, detach, and kill should all read from that registry.

This is no longer theoretical. In `src_v3`, the shared-session tests exposed exactly why log reconstruction and process-pattern cleanup are weak:

- stale jobs can survive a failed run
- a later run can accidentally attach to the wrong long-lived process
- room history can show lifecycle from the wrong command generation

The registry should own truth for:

- active vs done
- session ownership
- room subject
- job subject
- cleanup eligibility
- stop/kill status

That would let the REPL ask the registry to stop jobs cleanly instead of scraping `ps`, parsing text tables, or `pkill`-ing by regex.

## How to Simplify the Packages

## `src/dev.go`

What should remain:

- environment/bootstrap resolution
- ensure local runtime is reachable or start it
- publish normalized command envelope
- render returned events if running interactively

What should be removed:

- REPL-specific execution policy
- subtone-mode special cases scattered through wrapper logic
- duplicated config/env resolution
- log-routing policy

Desired end state:

- `dev.go` is a thin NATS client plus local bootstrap helper
- no `os.Exit(...)` paths that skip runtime cleanup; top-level command entrypoints should return exit codes after deferred cleanup has run

## `src/plugins/logs/src_v1`

What should remain:

- record creation
- sink implementations

What should be removed:

- REPL awareness
- prompt/prefix rules
- UI-context branching

Desired end state:

- logs emits structured records to sinks
- REPL subscribes and renders

## `src/plugins/test/src_v1`

What should remain:

- suite runner
- step runner
- normalized result model

What should be removed:

- too much transcript-specific logic in generic test runtime
- text-first matching as the primary assertion mode

Desired end state:

- tests subscribe to event subjects and assert structured envelopes
- rendered output assertions become optional smoke checks
- test harness has explicit suite-scoped fixtures for long-lived REPL sessions and separate isolated fixtures only for startup/bootstrap tests

## `src/plugins/repl/src_v3` -> `src_v4`

What should remain conceptually:

- multiplayer NATS room model
- index room vs job/subtone room split
- attach/detach UX

What should be removed:

- giant mixed runtime file
- command parsing coupled to rendering
- process state inferred from logs
- ad hoc room-history scanning

Desired end state:

- `protocol`: command/event envelope types and subject names
- `broker`: command dispatch and session coordination
- `registry`: job/subtone state
- `runner`: process execution and lifecycle updates
- `renderer`: local console formatting only
- `presence`: users/sessions/rooms

## Subject Model

Keep NATS central, but simplify the subject graph.

Recommended stable subjects:

- `dialtone.command`
- `dialtone.event`
- `dialtone.session.<session_id>`
- `dialtone.room.<room>`
- `dialtone.job.<job_id>`
- `dialtone.registry.job`

This is better than letting each feature invent its own subject family opportunistically.

For compatibility during migration, `src_v4` can still bridge the current `repl.room.*` and `repl.subtone.*` subjects, but the architectural goal should be one stable subject model instead of two parallel naming schemes.

## What v4 Should Not Copy From v3

1. Do not make room text the source of truth.
2. Do not use process pid as the user-facing handle.
3. Do not let wrapper diagnostics share the same semantic stream as REPL lifecycle.
4. Do not make tests infer state from recent transcript tails.
5. Do not keep separate command-routing logic in wrapper and REPL.
6. Do not make `subtone-list` reconstruct truth from log files when a live registry can exist.
7. Do not make suite reliability depend on process regexes being perfect.

## Migration Strategy

1. Keep `src_v3` green and frozen except for bug fixes.
2. Preserve the current suite-scoped REPL testing pattern as the expected user model for v4.
3. Define `v4` command envelope and event envelope first.
4. Implement a registry-backed job model before new renderer work.
5. Make one thin CLI client path publish commands to NATS.
6. Make one thin console renderer consume events from NATS.
7. Port `/help`, `/ps`, attach, detach, job listing, and stop/kill first.
8. Port external command execution second.
9. Port SSH and Cloudflare only after the local job model is stable.

## Success Criteria

`src_v4` should be considered better only if it materially reduces code and coupling.

That means:

- fewer implicit execution paths
- one command path through NATS
- one event model through NATS
- one long-lived session model for multiplayer REPL use
- registry-backed cleanup and job control
- no stale-process dependence between test runs
- one registry for job state
- renderer-only prompt logic
- tests that mostly assert structured events
- no log-reconstruction hacks for core runtime truth

If `src_v4` keeps the same amount of code but spreads it across more files, that is not success.

The real target is simpler behavior:

- many humans and LLM agents can share one REPL
- every running job is easy to list, attach, inspect, and kill
- logs are easy to separate into index vs job views
- failures are easy to debug from structured state, not transcript archaeology
