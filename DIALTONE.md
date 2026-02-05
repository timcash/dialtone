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
./dialtone.sh ticket list        # View the current Task DAG state
./dialtone.sh ticket start <id>  # Claim a task and begin collaboration
```

---

### Collaboration Log
A typical session showing `DIALTONE:` guiding `USER-1`, `LLM-reviewer`, and `LLM-tester` through a code change in `src/plugins/swarm`.

```shell
USER-1> ./dialtone.sh ticket start swarm-reconnect-logic
[DIALTONE] Loading Task DAG for [swarm-reconnect-logic]...
[DIALTONE] Current State: 1 Pending Subtask.
[DIALTONE] Assigned: USER-1 (Implementer), LLM-tester (Verification), LLM-reviewer (Audit).

USER-1> [Edit src/plugins/swarm/app/index.js: Fix reconnection backoff]
USER-1> ./dialtone.sh ticket subtask testcmd "npm run test:swarm"

LLM-tester> [ERROR] Test "reconnection-backoff" failed!
LLM-tester> Expected backoff: 2000ms, Received: 500ms.
LLM-tester> Trace: src/plugins/swarm/test/reconnect_test.js:42

USER-1> Ah, forgot the multiplier in the backoff loop.
USER-1> [Edit src/plugins/swarm/app/index.js: Line 142]
- const delay = baseDelay + attempt;
+ const delay = baseDelay * Math.pow(2, attempt);

USER-1> ./dialtone.sh ticket subtask start

LLM-tester> [SUCCESS] All tests passed.
LLM-tester> [LOG] swarm-simulation ... OK
LLM-tester> [LOG] reconnection-backoff ... OK

LLM-reviewer> Reviewing changes in src/plugins/swarm/app/index.js...
LLM-reviewer> Code looks clean. Backoff logic is compliant with engineering specs.
LLM-reviewer> Sign-off requested for Task DAG update.

USER-1> ./dialtone.sh ticket --sign swarm-reconnect-logic
[DIALTONE] Signature found: [USER-1]
[DIALTONE] Signature found: [LLM-reviewer]
[DIALTONE] All required signatures present. Task [swarm-reconnect-logic] marked as DONE.

USER-1> ./dialtone.sh ticket done swarm-reconnect-logic
[DIALTONE] Committing changes to git...
[DIALTONE] [LOG] commit 4f2a1b9: "fix(swarm): implement exponential backoff for reconnection"
[DIALTONE] Task completed successfully.

USER-1> I want to see the new swarm metrics in the UI. Open the dashboard.

[DIALTONE] Starting dashboard server at http://127.0.0.1:4000
[DIALTONE] Launching browser...
```