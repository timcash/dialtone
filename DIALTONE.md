# DIALTONE (Virtual Librarian)

`DIALTONE` is a virtual librarian that orchestrates a distributed mesh of humans and agents. It provides one interface for computation, engineering, design, logistics, geospatial analysis, graph analysis, and ML training workflows.
`RLM_DIALTONE.md` is the reference example transcript for recursive (RLM-style) operation.

## Core Model

### Mesh Network + Task DAG
- `Mesh Network`: Decentralized peers synchronize causal events over a P2P log stream (Hyperswarm).
- `Task DAG`: A directed acyclic graph of task events. Every task update, dependency, signature, and status transition is a timestamped causal entry.
- `Convergence`: All peers replay the same event log and converge on the same project state.

### Subtone Execution Model
- `DIALTONE` is a REPL-like command proxy over OS, network, database, and code tools.
- USER/LLM roles do not execute commands directly; they request execution with `@DIALTONE <command>`.
- Each executed request runs in a scoped subprocess called a `subtone`.
- Subtone stdout/stderr is streamed as `DIALTONE:PID:> ...` and persisted in the task log.
- `DIALTONE` can read/write files, run tests, invoke services, and query state, then attach outputs to the task DAG.

### Artifact + Signature Flow
- Inputs/outputs are tracked as task artifacts: `code`, `txt`, `markdown`, `json`, `image`, logs, and reports.
- LLMs and USER request artifact actions through DIALTONE (`artifact add`, `artifact upload`, `artifact link`).
- DIALTONE computes hashes, stores artifact metadata, and requests signatures before promote/publish operations.
- Artifact records include signer public keys (USER + participating LLM roles), signature status, and lineage.
- Signed artifacts are linked into task summaries so downstream LLMs can search and reuse verified files.

### Execution + Governance
- Commands are requested in chat and executed only after required signatures.
- `--blocked` signatures prevent execution until unblocked signatures satisfy policy.
- Task lifecycle is explicit: `pending -> in_progress -> done`.
- DIALTONE writes session summaries and asks participants to score criteria clarity, outcomes, and verification notes.

---

## Command Line Interface

```shell
# Core interaction
./dialtone.sh swarm status       # Check live peer count and network latency
./dialtone.sh swarm dashboard    # Launch visual network explorer
./dialtone.sh task list          # View current Task DAG state
./dialtone.sh task start <id>    # Claim task and begin collaboration
```

After `./dialtone.sh` starts, dialog becomes a single interactive stream:
- Commands no longer need `./dialtone.sh` prefix.
- DIALTONE requests `task --sign <task-name>` before running user/LLM command requests.
- USER and LLM roles can sign spontaneously, including `--blocked`.
- DIALTONE advances tasks when signature policy is satisfied.
- DIALTONE requests end-of-task review/sign-off and writes markdown reports.

---

## Collaboration Protocol (LLM-Searchable)

### Event Shape
Use one event per line:
- `ROLE> message`
- `DIALTONE:PID:> process output`
- Command execution is always requested as `@DIALTONE <command>`.
- USER/LLM roles request actions; only DIALTONE executes OS/network/db commands.

### Roles
- `USER-*`: requesters/operators
- `LLM-CODE`: implementation agent
- `LLM-TEST`: verification agent
- `LLM-REVIEW`: code audit agent
- `LLM-OPS`: deploy/runtime reliability agent
- `DIALTONE`: orchestrator and policy gate

### Signature Rules (Example Policy)
- Run command: requires at least one human signature and one unblocked agent signature on current task.
- Mark done: requires `--done` signatures from required roles.
- Blocked signatures are recorded and visible, but do not satisfy execution readiness.

---

## Session Transcript A: Reconnect Logic + UI Debug

### Phase 1: Task Start and Scope

```text
USER-1> @DIALTONE task start swarm-reconnect-logic
DIALTONE> Loading Task DAG for [swarm-reconnect-logic]...
DIALTONE> Current State: 1 pending subtask.
DIALTONE> Assigned: USER-1 (Requester), LLM-CODE (Implementation), LLM-TEST (Verification), LLM-REVIEW (Audit).
DIALTONE> SLA window: 5 minutes remain.
DIALTONE> Any side-tasks discovered? I can add them now if needed.

USER-1> Please fix the reconnection backoff and keep it aligned with the spec.
LLM-CODE> I can patch the backoff. Clarification: should this be exponential with a cap or pure exponential?
USER-1> Exponential with base 500ms, cap at 8s.
```

### Phase 2: First Test Run (Failure)

```text
LLM-CODE> [Edit src/plugins/swarm/app/index.js: update backoff formula + cap]
LLM-CODE> @DIALTONE npm run test:swarm
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic
LLM-TEST> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signatures verified. Running command via PID 3301...

DIALTONE:3301:> > dialtone-swarm@1.0.0 test:swarm
DIALTONE:3301:> > tap "test/reconnect_test.js"
DIALTONE:3301:>
DIALTONE:3301:> [FAIL] test/reconnect_test.js
DIALTONE:3301:> Expected backoff: 2000ms, received: 500ms.
DIALTONE:3301:> Trace: src/plugins/swarm/test/reconnect_test.js:42

DIALTONE> Process 3301 exited with code 1.

LLM-TEST> Test `reconnection-backoff` failed.
LLM-TEST> @USER Should we add jitter (+/-10%) to match the spec?
LLM-CODE> I will patch the formula and rerun tests.
```

### Phase 3: Second Test Run (Blocked Then Cleared)

```text
USER-1> Yes, include jitter.
LLM-CODE> [Edit src/plugins/swarm/app/index.js: add jitter + fix multiplier]

LLM-CODE> @DIALTONE npm run test:swarm
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic --blocked
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signature found: [USER-1] (blocked)
DIALTONE> Signature found: [LLM-REVIEW]
DIALTONE> Waiting for an unblocked execution signature.
LLM-CODE> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Execution policy satisfied (human signature present, unblocked agent signature present). Running command via PID 3345...

DIALTONE:3345:> > dialtone-swarm@1.0.0 test:swarm
DIALTONE:3345:> > tap "test/reconnect_test.js"
DIALTONE:3345:>
DIALTONE:3345:> [PASS] swarm-simulation ... OK
DIALTONE:3345:> [PASS] reconnection-backoff ... OK
DIALTONE:3345:>
DIALTONE:3345:> Tests completed successfully.

DIALTONE> Process 3345 exited with code 0.

LLM-TEST> [SUCCESS] All tests passed.
```

### Phase 4: Review + Done Signatures

```text
LLM-REVIEW> Reviewing changes in src/plugins/swarm/app/index.js...
LLM-REVIEW> Code looks clean. Backoff logic is compliant with engineering specs.
DIALTONE> Provide completion signatures with `--done` when ready.
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic --done
LLM-TEST> @DIALTONE task --sign swarm-reconnect-logic --done
LLM-CODE> @DIALTONE task --sign swarm-reconnect-logic --done
USER-1> @DIALTONE task --sign swarm-reconnect-logic --done
DIALTONE> All required `--done` signatures present. Task [swarm-reconnect-logic] marked DONE.
DIALTONE> Summary written to docs/reports/swarm-reconnect-logic.md
DIALTONE> Please review and score: criteria clarity, expected outcomes, and verification notes.
DIALTONE> Reminder: include `npm run test:swarm` evidence and open a PR when ready.

USER-1> Scores: criteria=4/5, outcomes=5/5, notes=clear
LLM-REVIEW> Scores: criteria=4/5, outcomes=4/5, notes=minor ambiguity on retry count
LLM-TEST> Scores: criteria=5/5, outcomes=5/5, notes=logs sufficient
LLM-CODE> Scores: criteria=4/5, outcomes=5/5, notes=implementation aligned
```

### Phase 5: Side Task Creation + Debug Follow-Up

```text
DIALTONE> Any side-tasks discovered? I can add them now if needed.
USER-1> Please start the dashboard in dev mode so I can validate the UI.
USER-1> @DIALTONE swarm dashboard --dev
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signatures verified. Starting dashboard server at http://127.0.0.1:4000 via PID 3402...
DIALTONE> Launching browser...

DIALTONE:3402:> [INFO] Server listening on port 4000
DIALTONE:3402:> [WARN] Metrics collector slow to start (200ms)

DIALTONE> Next: confirm UI state, capture errors, and queue follow-up tasks.

USER-1> I see a red error banner on the page. Add a debug task for this issue.
USER-1> @DIALTONE task add ui-error-banner-debug --type debug --parent swarm-reconnect-logic
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-banner-debug` to create and switch.
USER-1> @DIALTONE task --sign ui-error-banner-debug
LLM-REVIEW> @DIALTONE task --sign ui-error-banner-debug
DIALTONE> Task created and selected: [ui-error-banner-debug].
DIALTONE> Next: confirm repro steps and decide logs/tests.

LLM-TEST> I can inspect client logs and trace the error. Clarification: does banner appear on first load or after navigation?
USER-1> It appears on first load.
LLM-CODE> [Edit src/plugins/swarm/app/index.js: add debug logging around UI boot]
LLM-CODE> @DIALTONE swarm dashboard --dev
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-banner-debug` to run.
USER-1> @DIALTONE task --sign ui-error-banner-debug
LLM-CODE> @DIALTONE task --sign ui-error-banner-debug
DIALTONE> Signatures verified. Restarting dashboard in dev mode via PID 3410...

DIALTONE:3410:> [INFO] Server listening on port 4000
DIALTONE:3410:> [DEBUG] boot: UI initializing...
DIALTONE:3410:> [DEBUG] boot: config loaded

DIALTONE> Next: watch UI error logs and capture failing trace.

USER-1> The error is from missing UI error logs in dev mode.
DIALTONE> Detected related side-task. Next: add `ui-error-log-tap` or request search.
USER-1> @DIALTONE task add ui-error-log-tap --parent ui-error-banner-debug
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-log-tap` to create and switch.
USER-1> @DIALTONE task --sign ui-error-log-tap
LLM-REVIEW> @DIALTONE task --sign ui-error-log-tap
DIALTONE> Task created and selected: [ui-error-log-tap].

DIALTONE> Help: `task list`, `task start <id>`, `task add <id>`, `task --sign <id> [--blocked|--done]`, `swarm dashboard --dev`.
DIALTONE> Tip: request full-text search with `@DIALTONE "search <term>"`.
USER-1> @DIALTONE "search error banner"
DIALTONE> Search results: `src/plugins/swarm/app/index.js`, `src/plugins/swarm/docs/task-dag.md`.
DIALTONE> Reminder: periodic signature checks enabled; tests to consider: `npm run test:swarm`; PR required before merge.
```

---

## Session Transcript B: Pattern Detection + Cost-Adaptive Modeling

### Phase 1: Pattern Trigger

```text
USER-1> @DIALTONE task start media-catalog-latency
DIALTONE> Loading Task DAG for [media-catalog-latency]...
DIALTONE> Current State: 2 pending subtasks.
DIALTONE> Assigned: USER-1, LLM-CODE, LLM-TEST, LLM-REVIEW, LLM-OPS.
DIALTONE> Pattern trigger: `task-pattern/media-catalog-latency` matched in telemetry store.
DIALTONE> Cost signal: repeated classification requests on one product SKU family.
DIALTONE> Policy recommendation generated: `task add rlm-router-low-cost` and `task add vision-safety-vest-fast-detector`.
```

### Phase 2: Design Decision

```text
LLM-OPS> Cost telemetry shows 63% spend from repeated low-entropy prompts.
LLM-OPS> Recommendation: route repetitive prompts to compact model tier and keep complex tasks on primary model.
LLM-CODE> I can implement a task-router with confidence thresholds and fallback.
LLM-REVIEW> Add guardrails: if confidence < 0.82, escalate to primary model.
LLM-TEST> I will add drift checks and false-negative monitoring.
USER-1> Approve both: small RLM router plus fast image detection for item `safety-vest-orange`.

DIALTONE> Subtasks created: `rlm-router-low-cost`, `vision-safety-vest-fast-detector`.
DIALTONE> Next step: collect signatures to run training/data prep commands.
```

### Phase 3: Signed Execution

```text
LLM-CODE> @DIALTONE task --sign rlm-router-low-cost
USER-1> @DIALTONE task --sign rlm-router-low-cost
DIALTONE> Signatures verified. Running `train:rlm-router` via PID 3522...

DIALTONE:3522:> [INFO] ingesting 1.8M historical routing decisions
DIALTONE:3522:> [INFO] pruning token budget and recursive depth=2
DIALTONE:3522:> [INFO] validation cost delta: -41%
DIALTONE:3522:> [INFO] quality delta: -0.7% (within tolerance)

DIALTONE> Process 3522 exited with code 0.

LLM-CODE> @DIALTONE task --sign vision-safety-vest-fast-detector
USER-1> @DIALTONE task --sign vision-safety-vest-fast-detector
DIALTONE> Signatures verified. Running `train:vision-fast-detector --item safety-vest-orange` via PID 3530...

DIALTONE:3530:> [INFO] dataset: 42k labeled frames
DIALTONE:3530:> [INFO] target: high precision on single item class
DIALTONE:3530:> [INFO] precision=0.97 recall=0.91 latency=9ms/frame

DIALTONE> Process 3530 exited with code 0.
```

### Phase 4: Policy Outcome

```text
LLM-OPS> Deploy recommendation: enable adaptive routing for low-risk tasks only.
LLM-REVIEW> Require rollback guard and shadow-mode for 24h.
LLM-TEST> Add canary checks: cost/task, latency p95, and misroute rate.
DIALTONE> Policy recorded: adaptive compute enabled with canary + auto-fallback.
DIALTONE> Expected impact: lower inference cost with bounded quality risk.
```

---

## Session Transcript C: Publish Anomaly + Signed Rollback

### Phase 1: Post-Publish Detection

```text
USER-1> @DIALTONE www publish
DIALTONE> Request received. Sign with `@DIALTONE task --sign web-release-2026-02-13` to run.
USER-1> @DIALTONE task --sign web-release-2026-02-13
LLM-OPS> @DIALTONE task --sign web-release-2026-02-13
DIALTONE> Signatures verified. Running `www publish` via PID 3611...

DIALTONE:3611:> [INFO] build complete
DIALTONE:3611:> [INFO] deploy target=prod edge=global
DIALTONE:3611:> [WARN] error rate spike detected in region us-east-1
DIALTONE:3611:> [WARN] checkout API 5xx above threshold

DIALTONE> Process 3611 exited with code 0.
LLM-OPS> Noticing anomaly in logs after `www publish`. Recommend immediate rollback.
```

### Phase 2: Rollback Decision + Signatures

```text
DIALTONE> Proposed action: `www rollback --to release-2026-02-12.4`.
DIALTONE> Reason: protect checkout SLO while root-cause analysis runs.
LLM-REVIEW> I support rollback. Risk of waiting is higher than rollback cost.
USER-1> Approve rollback.

DIALTONE> Sign rollback command with `task --sign web-release-2026-02-13-rollback`.
USER-1> @DIALTONE task --sign web-release-2026-02-13-rollback
LLM-OPS> @DIALTONE task --sign web-release-2026-02-13-rollback
LLM-REVIEW> @DIALTONE task --sign web-release-2026-02-13-rollback
DIALTONE> Signatures verified. Executing rollback via PID 3620...
```

### Phase 3: Rollback Execution + Closeout

```text
DIALTONE:3620:> [INFO] rolling back to release-2026-02-12.4
DIALTONE:3620:> [INFO] traffic shifted: 10% -> 50% -> 100%
DIALTONE:3620:> [INFO] checkout API 5xx returned to baseline

DIALTONE> Process 3620 exited with code 0.
DIALTONE> Incident status: mitigated.
DIALTONE> Next: open `postmortem-web-release-2026-02-13`, attach diff/logs, and gate republish on fix validation.

LLM-OPS> @DIALTONE task --sign web-release-2026-02-13-rollback --done
LLM-REVIEW> @DIALTONE task --sign web-release-2026-02-13-rollback --done
USER-1> @DIALTONE task --sign web-release-2026-02-13-rollback --done
DIALTONE> Rollback task marked DONE. Summary written to docs/reports/web-release-2026-02-13-rollback.md
```

---

## Design Notes for Future Dialog Generation

- Keep actor prefixes stable for parsing (`ROLE>`).
- Keep command requests and signature gates explicit.
- Emit PID-scoped process logs as separate lines (`DIALTONE:PID:>`).
- Record why a command is blocked/unblocked.
- Require explicit `--done` signatures for task completion.
- For adaptive compute, log: trigger, policy, thresholds, measured deltas, and fallback.
