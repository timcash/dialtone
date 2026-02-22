# REPL Task Log Workflow (`DIALTONE>`)

This document defines the REPL-first task workflow log format.

It replaces task context switching with a deterministic dependency loop:
- one `DIALTONE>` loop over all tasks
- `task next` picks the next actionable task by dependency order
- each task is reviewed field-by-field
- roles (`LLM-*`, `USER-*`) sign in `### signatures:` before moving on

---

## 1) Core Model

`DIALTONE>` behaves like a for-loop over the task DAG:
1. find next task whose `inputs` are done
2. run checklist over required fields (`description`, `docs`, `test-condition`, `test-command`, `reviewed`, `tested`, `signatures`)
3. ask required roles to sign
4. mark task done
5. continue until root task is done

No `task start <subtask>` context switches are required.

---

## 2) REPL Commands

- `DIALTONE> task next`
  - returns the next actionable task (dependencies satisfied, not done)
- `DIALTONE> task show <task-id>`
  - prints key fields + current signatures
- `DIALTONE> task sign <task-id> --role <ROLE>`
  - appends role signature; also routes to `reviewed`/`tested` sections
- `DIALTONE> task validate <task-id>`
  - validates task markdown structure
- `DIALTONE> task tree <root-id>`
  - shows dependency order

Recommended role set per task:
- `LLM-CODE`
- `LLM-TEST`
- `LLM-REVIEW`
- optional: `LLM-DOCS`, `USER-1`, `USER-2`

---

## 3) Log Format

```text
DIALTONE> <system step / command>
LLM-*> <analysis or action>
USER-*> <decision / approval>
DIALTONE:<PID>> <subprocess output>
```

All command output should be logged through the logs system (no raw stdout printing in plugin code).

---

## 4) Deterministic `task next`

`task next` should evaluate:
1. `inputs` all done?
2. required fields present and non-empty?
3. test conditions and commands reviewed?
4. role signatures present in `### signatures:`?
5. if yes, task done; else return task for completion/signing

Suggested ordering:
- topological order by `inputs`
- stable tiebreaker: lexical task id

---

## 5) Example: 3 Tasks Then Root

Assume root `104` with inputs:
- `104-docs`
- `104-tests`
- `104-impl`

### Session

```text
DIALTONE> task tree 104
DIALTONE> - 104
DIALTONE>   - 104-docs
DIALTONE>   - 104-tests
DIALTONE>   - 104-impl

DIALTONE> task next
DIALTONE> NEXT: 104-docs (inputs satisfied)

LLM-DOCS> Updated documentation links and acceptance notes in 104-docs.
LLM-DOCS> @DIALTONE task sign 104-docs --role DOCS
LLM-REVIEW> @DIALTONE task sign 104-docs --role REVIEW
LLM-TEST> @DIALTONE task sign 104-docs --role TEST

DIALTONE> task validate 104-docs
DIALTONE> PASS
DIALTONE> Task 104-docs marked done.

DIALTONE> task next
DIALTONE> NEXT: 104-tests (inputs satisfied)

LLM-TEST> Ran test-command and updated test-condition evidence.
LLM-TEST> @DIALTONE task sign 104-tests --role TEST
LLM-REVIEW> @DIALTONE task sign 104-tests --role REVIEW
USER-1> @DIALTONE task sign 104-tests --role USER-1

DIALTONE> task validate 104-tests
DIALTONE> PASS
DIALTONE> Task 104-tests marked done.

DIALTONE> task next
DIALTONE> NEXT: 104-impl (inputs satisfied)

LLM-CODE> Implemented changes and linked artifacts.
LLM-CODE> @DIALTONE task sign 104-impl --role CODE
LLM-TEST> @DIALTONE task sign 104-impl --role TEST
LLM-REVIEW> @DIALTONE task sign 104-impl --role REVIEW

DIALTONE> task validate 104-impl
DIALTONE> PASS
DIALTONE> Task 104-impl marked done.

DIALTONE> task next
DIALTONE> NEXT: 104 (all inputs done)

LLM-REVIEW> Reviewed root fields: description/docs/tests/inputs/outputs/signatures.
LLM-TEST> Verified root test-condition and test-command evidence.
LLM-REVIEW> @DIALTONE task sign 104 --role REVIEW
LLM-TEST> @DIALTONE task sign 104 --role TEST
USER-1> @DIALTONE task sign 104 --role USER-1

DIALTONE> task validate 104
DIALTONE> PASS
DIALTONE> task resolve 104 --pr-url https://github.com/<org>/<repo>/pull/999
DIALTONE> Root task 104 completed. Issue sync updated.

DIALTONE> task next
DIALTONE> No remaining actionable tasks.
```

---

## 6) Multi-Agent Parallelism With One Loop

You can work on multiple tasks at once while keeping one deterministic coordinator:
- `LLM-CODE` works implementation task
- `LLM-TEST` works test task
- `LLM-DOCS` works docs task
- `USER-*` roles provide approvals/signatures

`DIALTONE>` still advances one deterministic step at a time via `task next`, so progress is auditable and reproducible.

