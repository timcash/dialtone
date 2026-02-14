# RLM DIALTONE (Recursive Language Model Orchestration Example)

This file is an example of RLM-style dialog for DIALTONE.
It is intentionally operational: LLM/USER roles can run lightweight local commands and edit files directly, while DIALTONE handles DAG/mesh/signature orchestration through `subtones`.

## RLM Basics (Paper-Oriented)

The RLM paper frames reasoning as an interactive loop with an external environment rather than a single long in-context pass.
Core ideas used here:
- `Environment state first`: retrieve only the next needed slices (files, logs, metrics), not full history dumps.
- `Recursive decomposition`: break a task into smaller scoped calls and re-enter the loop with updated state.
- `Bounded feedback`: return compact outputs from each recursive step to reduce context growth and cost.
- `Convergence by execution`: use environment checks (tests, metrics, constraints) to validate progress.
- `Policy adaptation`: feed observed outcomes back into routing/depth/summarization policy.

## Domain Language

Use this compact language in DIALTONE RLM tasks:
- `observe`: sample environment state (`files`, `logs`, `metrics`, `DAG`) needed for the next step.
- `decompose`: create recursive subtasks with bounded scope and explicit outputs.
- `execute`: run command/action locally or in a subtone.
- `ingest`: register a local artifact into DAG with hash + metadata.
- `relay`: broadcast DAG events/artifacts to mesh peers with ack tracking.
- `sign`: attach signer public keys and signature records to artifacts/tasks.
- `verify`: run tests/checks against objective constraints.
- `adapt`: update policy from measured deltas and failure modes.
- `converge`: mark task objective satisfied with signed evidence.

## Glossary

| Term | Meaning in DIALTONE RLM |
| :--- | :--- |
| `environment state` | External working memory: files, logs, metrics, DAG entries. |
| `recursive step` | One bounded subproblem solved before returning compact output. |
| `bounded feedback` | Token-limited summary/trace emitted by a step for the next step. |
| `subtone` | PID-scoped execution process started by DIALTONE. |
| `artifact` | Any task file output (`code`, `txt`, `markdown`, `json`, `image`, logs). |
| `artifact ingest` | DAG registration of a local artifact with hash, type, lineage, and links. |
| `mesh relay` | Peer-to-peer propagation of DAG events/artifacts with acknowledgements. |
| `signature record` | Public-key signature metadata for USER/LLM approvals and provenance. |
| `policy loop` | Measure -> adjust routing/depth/summarization -> re-measure. |
| `convergence check` | Objective pass/fail signal from tests, constraints, or runtime metrics. |

## Comparison Against arXiv:2512.24601v2

The paper emphasizes an environment-driven recursive loop where the model interacts with external state through a bounded interface, then re-enters reasoning with compact context. This example aligns with that direction:
- `Externalized state`: files, logs, metrics, and task DAG are treated as environment state, not full in-context dumps.
- `Recursive calls`: DIALTONE spawns sub-RLM tasks (`RLM-*`) for scoped subtasks.
- `Bounded feedback`: each subtone returns compact outputs and metrics for the next step.
- `Policy updates`: outcomes feed back into routing/prompt policies.

What is still partial in this repo (explicitly not fully implemented yet):
- no full VM-like symbolic machine with strict variable semantics for every operation
- limited automatic recursion-depth control and branch pruning heuristics
- partial convergence checks (task-level today, not full formal proof/objective checks)

---

## RLM Operating Rules in DIALTONE

- Lightweight local ops can run directly from USER/LLM roles (`cat`, `rg`, `ls`, editors, basic tests).
- DAG registration, mesh sync, artifact signing, and privileged ops run via `@DIALTONE <command>`.
- DIALTONE runs commands in `subtones` and emits output as `DIALTONE:PID:>`.
- LLM/USER edits become artifacts; DIALTONE ingests, hashes, links, and requests signatures.
- Promote/publish steps require signatures from USER and required LLM roles.

---

## Session Transcript: Improving DIALTONE with RLM Patterns

### Phase 1: Boot + Policy Pattern Match

```text
USER-1> @DIALTONE task start rlm-process-upgrade-v1
DIALTONE> Loading Task DAG for [rlm-process-upgrade-v1]...
DIALTONE> Assigned: USER-1, LLM-CODE, LLM-TEST, LLM-REVIEW, LLM-OPS.
DIALTONE> Policy match: `rlm.pattern.recursive_environment_loop=true`.
DIALTONE> Created subtasks: `rlm-env-index`, `rlm-artifact-signing`, `rlm-metric-loop`.

LLM-OPS> @DIALTONE task metrics rlm-process-upgrade-v1 --window 7d
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-process-upgrade-v1` to run.
USER-1> @DIALTONE task --sign rlm-process-upgrade-v1
LLM-OPS> @DIALTONE task --sign rlm-process-upgrade-v1
DIALTONE> Signatures verified. Running command via PID 5011...
DIALTONE:5011:> [METRIC] avg_context_tokens=18120
DIALTONE:5011:> [METRIC] rerun_rate=0.37
DIALTONE:5011:> [METRIC] task_cost_usd=142.88
DIALTONE> Process 5011 exited with code 0.
```

### Phase 2: LLMs Read Files and Edit Locally

```text
LLM-CODE> rg -n "Subtone Execution Model|Artifact \\+ Signature Flow" DIALTONE.md
LLM-CODE> sed -n '1,120p' DIALTONE.md
LLM-REVIEW> sed -n '1,220p' RLM_DIALTONE.md
LLM-TEST> rg -n "partial|bounded feedback|recursive" RLM_DIALTONE.md
LLM-REVIEW> curl -s https://arxiv.org/html/2512.24601v2 | rg -n "recursive|environment|feedback"

LLM-CODE> [Edit .dialtone/policy/rlm_routing.yaml: add bounded_feedback_tokens=512 and depth policy]
LLM-TEST> [Edit docs/rlm/convergence_checks.md: add DAG artifact integrity checks]
LLM-OPS> [Edit docs/rlm/mesh_relay.md: add peer relay/ack flow]
```

### Phase 3: DIALTONE Ingests Artifacts into DAG and Relays Mesh Data

```text
LLM-CODE> @DIALTONE artifact ingest --task rlm-process-upgrade-v1 --path .dialtone/policy/rlm_routing.yaml --kind yaml
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-CODE> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running artifact ingest via PID 5044...
DIALTONE:5044:> [ARTIFACT] id=a19 path=.dialtone/policy/rlm_routing.yaml sha256=4d...92 bytes=1298
DIALTONE:5044:> [DAG] append event=artifact.ingest id=a19 task=rlm-process-upgrade-v1
DIALTONE> Process 5044 exited with code 0.

LLM-TEST> @DIALTONE artifact ingest --task rlm-process-upgrade-v1 --path docs/rlm/convergence_checks.md --kind markdown
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-TEST> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running artifact ingest via PID 5052...
DIALTONE:5052:> [ARTIFACT] id=a20 path=docs/rlm/convergence_checks.md sha256=ad...0f bytes=2114
DIALTONE:5052:> [DAG] append event=artifact.ingest id=a20 task=rlm-process-upgrade-v1
DIALTONE> Process 5052 exited with code 0.

LLM-OPS> @DIALTONE artifact ingest --task rlm-process-upgrade-v1 --path docs/rlm/mesh_relay.md --kind markdown
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-OPS> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running artifact ingest via PID 5058...
DIALTONE:5058:> [ARTIFACT] id=a21 path=docs/rlm/mesh_relay.md sha256=8a...6c bytes=1766
DIALTONE:5058:> [DAG] append event=artifact.ingest id=a21 task=rlm-process-upgrade-v1
DIALTONE> Process 5058 exited with code 0.

LLM-OPS> @DIALTONE mesh relay --dag-event artifact.ingest --task rlm-process-upgrade-v1 --peers 6
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-OPS> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running mesh relay via PID 5072...
DIALTONE:5072:> [MESH] relay_start peers=6 events=3
DIALTONE:5072:> [MESH] peer=12D3KooW... ack artifact=a19
DIALTONE:5072:> [MESH] peer=12D3KooX... ack artifact=a20
DIALTONE:5072:> [MESH] peer=12D3KooY... ack artifact=a21
DIALTONE:5072:> [MESH] relay_complete acked=6/6
DIALTONE> Process 5072 exited with code 0.

LLM-REVIEW> @DIALTONE artifact sign --task rlm-process-upgrade-v1 --artifacts a19,a20,a21
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-REVIEW> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running signature workflow via PID 5076...
DIALTONE:5076:> [SIGN] a19 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_code_2a...
DIALTONE:5076:> [SIGN] a20 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_test_7b...
DIALTONE:5076:> [SIGN] a21 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_ops_5d...
DIALTONE:5076:> [DAG] append event=artifact.signed artifacts=[a19,a20,a21]
DIALTONE> Process 5076 exited with code 0.
```

### Phase 5: Apply Patterns and Show Improvement

```text
LLM-CODE> @DIALTONE task apply-policy rlm-process-upgrade-v1 --file .dialtone/policy/rlm_routing.yaml
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-metric-loop` to run.
USER-1> @DIALTONE task --sign rlm-metric-loop
LLM-REVIEW> @DIALTONE task --sign rlm-metric-loop
DIALTONE> Signatures verified. Running command via PID 5084...
DIALTONE:5084:> [POLICY] recursive_decompose=enabled
DIALTONE:5084:> [POLICY] max_recursion_depth=2
DIALTONE:5084:> [POLICY] summarize_before_recurse=true
DIALTONE:5084:> [POLICY] bounded_feedback_tokens=512
DIALTONE> Process 5084 exited with code 0.

LLM-OPS> @DIALTONE task metrics rlm-process-upgrade-v1 --window 24h
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-metric-loop` to run.
USER-1> @DIALTONE task --sign rlm-metric-loop
LLM-OPS> @DIALTONE task --sign rlm-metric-loop
DIALTONE> Signatures verified. Running command via PID 5091...
DIALTONE:5091:> [METRIC] avg_context_tokens=10340
DIALTONE:5091:> [METRIC] rerun_rate=0.21
DIALTONE:5091:> [METRIC] task_cost_usd=87.12
DIALTONE:5091:> [DELTA] token_reduction=-42.9% rerun_reduction=-43.2% cost_reduction=-39.0%
DIALTONE> Process 5091 exited with code 0.

DIALTONE> Improvement recorded in task state. Next: continue rollout with canary policy and signed artifact promotion.
USER-1> @DIALTONE task --sign rlm-process-upgrade-v1 --done
LLM-CODE> @DIALTONE task --sign rlm-process-upgrade-v1 --done
LLM-TEST> @DIALTONE task --sign rlm-process-upgrade-v1 --done
LLM-REVIEW> @DIALTONE task --sign rlm-process-upgrade-v1 --done
LLM-OPS> @DIALTONE task --sign rlm-process-upgrade-v1 --done
DIALTONE> All done signatures present. Summary written to docs/reports/rlm-process-upgrade-v1.md
```

---

## Minimal Vocabulary

| Term | DIALTONE Usage |
| :--- | :--- |
| `subtone` | PID-scoped execution process for one requested command. |
| `environment state` | Files/logs/metrics/task DAG sampled by DIALTONE for the next step. |
| `recursive call` | Spawned subtask (`RLM-*`) with bounded scope and returned artifact/metric output. |
| `artifact ingest` | Register local file into DAG with hash, metadata, and lineage. |
| `mesh relay` | Broadcast DAG events/artifacts across peers with ACK tracking. |
| `policy loop` | Measure results, write policy, apply policy, and re-measure. |
