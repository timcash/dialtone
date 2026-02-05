# DIALTONE (Virtual Librarian)

`DIALTONE:` is a **virtual librarian** that orchestrates a distributed mesh network of agents and humans. It provides a single interface for complex domains: computation, engineering, design, logistics, geospatial analysis, graph analysis, and machine learning training environments.

### The Mesh & The Task DAG
System coordination happens over a peer-to-peer **mesh log stream**. 
- **Mesh Network**: A decentralized network of nodes (peers) that sync data using P2P protocols (Hyperswarm).
- **Task DAG**: A Directed Acyclic Graph formed by log entries. Every task, dependency, and signature is a causal event in a global timeline, allowing all peers to converge on the same project state.

---

### Command Line Interface
```shell
# Core interaction
./dialtone.sh swarm status      # Check live peer counts and network latency
./dialtone.sh swarm dashboard   # Launch the visual network explorer
./dialtone.sh task list          # View the current Task DAG state
./dialtone.sh task start <id>    # Claim a task and begin collaboration
```

After `./dialtone.sh` starts, the dialog becomes a single interactive stream.
Commands no longer need the `./dialtone.sh` prefix; `DIALTONE` prints context,
asks for `--sign <task-name>` before running commands on behalf of the user or
LLM, then reports results and writes a summary to a markdown file. `DIALTONE`
occasionally requests signatures on task status. The USER and LLMs can also
sign spontaneously with `task --sign <task-name>` (including `--blocked`) and
`DIALTONE` advances the task when all required signatures are present. It also
asks the USER and LLMs to review the summary, score criteria clarity and
outcomes, and sign off that each role completed their part.

---

### Collaboration Log
A typical session showing `DIALTONE:` guiding `USER-1`, `LLM-CODE`, `LLM-REVIEW`, and `LLM-TEST` through a code change in `src/plugins/swarm`.

```shell
USER-1> ./dialtone.sh task start swarm-reconnect-logic
DIALTONE> Loading Task DAG for [swarm-reconnect-logic]...
DIALTONE> Current State: 1 Pending Subtask.
DIALTONE> Assigned: USER-1 (Requester), LLM-CODE (Implementation), LLM-TEST (Verification), LLM-REVIEW (Audit).
DIALTONE> Any side-tasks discovered? I can add them now if needed.

USER-1> Please fix the reconnection backoff and keep it aligned with the spec.
LLM-CODE> I can patch the backoff. Clarification: should this be exponential with a cap or pure exponential?
USER-1> Exponential with base 500ms, cap at 8s.

LLM-CODE> [Edit src/plugins/swarm/app/index.js: Update backoff formula + cap]
LLM-CODE> @DIALTONE `npm run test:swarm`
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic
LLM-TEST> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signatures verified. Running command and streaming logs to all peers...
DIALTONE> Next: confirm expected backoff behavior in the logs.

[ERROR]
LLM-TEST> Test "reconnection-backoff" failed!
LLM-TEST> Expected backoff: 2000ms, Received: 500ms.
LLM-TEST> Trace: src/plugins/swarm/test/reconnect_test.js:42
LLM-TEST> @USER Should we add jitter (±10%) to match the spec?
LLM-CODE> I'll patch the formula and re-run the tests.

USER-1> Yes, include jitter.
LLM-CODE> [Edit src/plugins/swarm/app/index.js: Add jitter + fix multiplier]

LLM-CODE> @DIALTONE `npm run test:swarm`
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic --blocked
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signature found: [USER-1] (blocked)
DIALTONE> Signature found: [LLM-REVIEW]
DIALTONE> Waiting on an unblocked signature to run the command.
LLM-CODE> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signatures verified. Running command and streaming logs to all peers...

LLM-TEST> [SUCCESS] All tests passed.
LLM-TEST> [LOG] swarm-simulation ... OK
LLM-TEST> [LOG] reconnection-backoff ... OK

LLM-REVIEW> Reviewing changes in src/plugins/swarm/app/index.js...
LLM-REVIEW> Code looks clean. Backoff logic is compliant with engineering specs.
DIALTONE> Please provide status signatures for `--done` when ready.
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic
LLM-TEST> @DIALTONE task --sign swarm-reconnect-logic
LLM-CODE> @DIALTONE task --sign swarm-reconnect-logic
USER-1> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> All required signatures present. Task [swarm-reconnect-logic] marked as DONE.
DIALTONE> Summary written to docs/reports/swarm-reconnect-logic.md
DIALTONE> Please review and score: test criteria clarity, expected outcomes, and verification notes.
DIALTONE> Reminder: tests should include `npm run test:swarm` and a PR is needed when ready.

USER-1> Scores: criteria=4/5, outcomes=5/5, notes=clear
LLM-REVIEW> Scores: criteria=4/5, outcomes=4/5, notes=minor ambiguity on retry count
LLM-TEST> Scores: criteria=5/5, outcomes=5/5, notes=logs sufficient
LLM-CODE> Scores: criteria=4/5, outcomes=5/5, notes=implementation aligned

DIALTONE> Any side-tasks discovered? I can add them now if needed.
USER-1> Please start the dashboard in dev mode so I can validate the UI.
USER-1> @DIALTONE `swarm dashboard --dev`
DIALTONE> Request received. Sign with `@DIALTONE task --sign swarm-reconnect-logic` to run.
USER-1> @DIALTONE task --sign swarm-reconnect-logic
LLM-REVIEW> @DIALTONE task --sign swarm-reconnect-logic
DIALTONE> Signatures verified. Starting dashboard server at http://127.0.0.1:4000
DIALTONE> Launching browser...
DIALTONE> Next: confirm UI state, capture any errors, and queue follow-up tasks.

USER-1> I see a red error banner on the page. Add a debug task for this issue.
USER-1> @DIALTONE `task add ui-error-banner-debug --type debug --parent swarm-reconnect-logic`
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-banner-debug` to create and switch.
USER-1> @DIALTONE task --sign ui-error-banner-debug
LLM-REVIEW> @DIALTONE task --sign ui-error-banner-debug
DIALTONE> Task created and selected: [ui-error-banner-debug].
DIALTONE> Next: confirm repro steps and decide which logs/tests to run.

LLM-TEST> I can inspect the client logs and trace the error. Clarification: does the banner appear on first load or after navigation?
USER-1> It appears on first load.
LLM-CODE> [Edit src/plugins/swarm/app/index.js: Add debug logging around UI boot]
LLM-CODE> @DIALTONE `swarm dashboard --dev`
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-banner-debug` to run.
USER-1> @DIALTONE task --sign ui-error-banner-debug
LLM-CODE> @DIALTONE task --sign ui-error-banner-debug
DIALTONE> Signatures verified. Restarting dashboard in dev mode and streaming logs...
DIALTONE> Next: watch UI error logs and capture failing trace.

USER-1> The error is from missing UI error logs in dev mode.
DIALTONE> Detected a related side-task. Next: add `ui-error-log-tap` or request help/search.
USER-1> @DIALTONE `task add ui-error-log-tap --parent ui-error-banner-debug`
DIALTONE> Request received. Sign with `@DIALTONE task --sign ui-error-log-tap` to create and switch.
USER-1> @DIALTONE task --sign ui-error-log-tap
LLM-REVIEW> @DIALTONE task --sign ui-error-log-tap
DIALTONE> Task created and selected: [ui-error-log-tap].

DIALTONE> Help: `task list`, `task start <id>`, `task add <id>`, `task --sign <id> [--blocked]`, `swarm dashboard --dev`.
DIALTONE> Tip: request full-text search with `@DIALTONE "search <term>"`.
USER-1> @DIALTONE "search error banner"
DIALTONE> Search results: `src/plugins/swarm/app/index.js`, `src/plugins/swarm/docs/task-dag.md`.
DIALTONE> Reminder: pending `--sign` updates will be requested on interval; tests to consider: `npm run test:swarm`; PR required before merge.
```
```