# RLM DIALTONE (Recursive Language Model Orchestration Example)

This file is an example of RLM-style dialog for DIALTONE.
It is intentionally operational: DIALTONE is a REPL/proxy that executes commands in `subtones`, while USER/LLM roles request actions with `@DIALTONE`.

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

- All execution is via `@DIALTONE <command>`.
- DIALTONE runs commands in `subtones` and emits output as `DIALTONE:PID:>`.
- LLM/USER edits become artifacts; DIALTONE uploads, hashes, links, and requests signatures.
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

### Phase 2: LLMs Read Files Through DIALTONE

```text
LLM-CODE> @DIALTONE fs read DIALTONE.md --lines 1:120
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-env-index` to run.
USER-1> @DIALTONE task --sign rlm-env-index
LLM-CODE> @DIALTONE task --sign rlm-env-index
DIALTONE> Signatures verified. Running command via PID 5023...
DIALTONE:5023:> # DIALTONE (Virtual Librarian)
DIALTONE:5023:> ...
DIALTONE:5023:> ### Subtone Execution Model
DIALTONE> Process 5023 exited with code 0.

LLM-REVIEW> @DIALTONE fs read RLM_DIALTONE.md --lines 1:220
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-env-index` to run.
USER-1> @DIALTONE task --sign rlm-env-index
LLM-REVIEW> @DIALTONE task --sign rlm-env-index
DIALTONE> Signatures verified. Running command via PID 5031...
DIALTONE:5031:> # RLM DIALTONE (Recursive Language Model Orchestration Example)
DIALTONE:5031:> ...
DIALTONE:5031:> What is still partial in this repo...
DIALTONE> Process 5031 exited with code 0.
```

### Phase 3: LLMs Write Policy + Prompt Artifacts

```text
LLM-CODE> @DIALTONE fs write .dialtone/policy/rlm_routing.yaml --from-stdin
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-CODE> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running command via PID 5044...
DIALTONE:5044:> [WRITE] .dialtone/policy/rlm_routing.yaml bytes=1298
DIALTONE> Process 5044 exited with code 0.

LLM-TEST> @DIALTONE fs write docs/rlm/convergence_checks.md --from-stdin
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-TEST> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running command via PID 5052...
DIALTONE:5052:> [WRITE] docs/rlm/convergence_checks.md bytes=2114
DIALTONE> Process 5052 exited with code 0.

LLM-OPS> @DIALTONE image annotate assets/rlm-loop.png --label "env->subtone->artifact->sign->policy"
DIALTONE> Request received. Sign with `@DIALTONE task --sign rlm-artifact-signing` to run.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-OPS> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running command via PID 5058...
DIALTONE:5058:> [WRITE] assets/rlm-loop-annotated.png bytes=84321
DIALTONE> Process 5058 exited with code 0.
```

### Phase 4: Upload + Public-Key Signatures

```text
LLM-CODE> @DIALTONE artifact upload --task rlm-process-upgrade-v1 --path .dialtone/policy/rlm_routing.yaml --kind yaml
LLM-TEST> @DIALTONE artifact upload --task rlm-process-upgrade-v1 --path docs/rlm/convergence_checks.md --kind markdown
LLM-OPS> @DIALTONE artifact upload --task rlm-process-upgrade-v1 --path assets/rlm-loop-annotated.png --kind image
DIALTONE> Upload requests queued. Sign with `@DIALTONE task --sign rlm-artifact-signing` to ingest artifacts.
USER-1> @DIALTONE task --sign rlm-artifact-signing
LLM-CODE> @DIALTONE task --sign rlm-artifact-signing
LLM-TEST> @DIALTONE task --sign rlm-artifact-signing
LLM-OPS> @DIALTONE task --sign rlm-artifact-signing
DIALTONE> Signatures verified. Running artifact ingest via PID 5072...
DIALTONE:5072:> [ARTIFACT] id=a19 path=.dialtone/policy/rlm_routing.yaml sha256=4d...92
DIALTONE:5072:> [ARTIFACT] id=a20 path=docs/rlm/convergence_checks.md sha256=ad...0f
DIALTONE:5072:> [ARTIFACT] id=a21 path=assets/rlm-loop-annotated.png sha256=8a...6c
DIALTONE:5072:> [SIGN] a19 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_code_2a...
DIALTONE:5072:> [SIGN] a20 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_test_7b...
DIALTONE:5072:> [SIGN] a21 user_pk=ed25519:usr_9f... llm_pk=ed25519:llm_ops_5d...
DIALTONE:5072:> [LINK] task=rlm-process-upgrade-v1 artifacts=[a19,a20,a21]
DIALTONE> Process 5072 exited with code 0.
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
| `artifact ingest` | Upload + hash + signer public keys + task linkage. |
| `policy loop` | Measure results, write policy, apply policy, and re-measure. |
