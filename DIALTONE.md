# DIALTONE (Virtual Librarian)

`DIALTONE:` is a **virtual librarian** that combines a task database, multimedia search engine, global messaging infrastructure, robotic controls and LLM CLI tools into a single interface.

`DIALTONE:`
- an `issue` is any multimodal prompt input into `./dialtone.sh` 
- after review an `issue` becomes an `outline` which becomes a `ticket`
- programtically explores a task directed asiclic graph (DAG)
- writes parquet files for flexible and strutured logs
- assigns scores to `LLM:` and `USER:` created plan outlines
- uses the concept of a `budget`, `graph-cost` and `success_probability`
- routes subtasks to multiple `LLM:` and `USER:` for relative comparison
- injects `subtask` lists into new tickets to act as `workflows`
- wraps duckdb for larger than memeory analysis and search
- dynamically writes context to files with a subtask and `--sign` key
- works with `LLM:` and `USER:` to craft CLI `commands`
- executes `commands` then proxies to `LLM:` and `USER:`
- is designed to bootstrap itself up from a shell file
- uses Golang as a simple but proven at scale interface to computation
- uses TDD to mananage large codebases
- uses NATS as a message bus and data infrastructure interface


### Example Log

```xml
<transcript>
  <turn>
    <user>turn this issue into a ticket `src/issues/0039.md`</user>
    <llm>
      <action>Reading the issue `src/issues/0039.md`</action>
      <reasoning>The issue is about adding a section to the www site that shows the Earth spinning. It appears to have enough information to become a ticket. We should start in add mode.</reasoning>
      <command>./dialtone.sh ticket add www-dev-page-earth-spin</command>
    </llm>
    <dialtone-response id="dt-3X4FT-449NV">
      <message-length>1024 bytes</message-length>
      <mode>add</mode>
      <context>
        <item>the ticket has been added with relavant files</item>
        <item>the files at are `src/tickets/www-dev-page-earth-spin/`</item>
        <item>before `start` or `review` check other tickets for similarity</item>
        <item>decide if this issue should belong to an existing open ticket</item>
        <item>if so decied if this ticket should be maked as a duplicate</item>
        <item>use this reference document: `docs/workflows/issue-similarity.md`</item>
      </context>
      <next-commands>
        <command>./dialtone.sh ticket list --sign dt-3X4FT-449NV</command>
      </next-commands>
    </dialtone-response>
  </turn>
  <turn>
    <assistant>
      <llm-reasoning>The ticket has been added but I need to review other tickets to ensure there is no duplication. I must use the `--sign` to advance the state of the ticket.</llm-reasoning>
      <llm-command>./dialtone.sh ticket list --sign dt-3X4FT-449NV</llm-command>
    </assistant>
    <dialtone id="dt-8VCCD-332HN">
      <message-length>2048 bytes</message-length>
      <mode>list</mode>
      <context>A ticket summary list has been created. Review them for similarity or to potentially add this issue to as a subtask of an existing ticket.</context>
      <resource path="src/tickets/ticket-list-summary.md"/>
      <next-steps>
        <option type="yes">./dialtone.sh ticket --sign dt-no-8VCCD-332HN</option>
        <option type="no">./dialtone.sh ticket --sign dt-yes-8VCCD-332HN</option>
      </next-steps>
    </dialtone>
  </turn>
  <turn>
    <assistant>
      <llm-reasoning>After reviewing the ticket list, I have determined that there are similar tickets. I will now link this issue as a subtask to the most relevant ticket.</llm-reasoning>
      <llm-command>./dialtone.sh ticket --similar dt-8VCCD-332HN</llm-command>
    </assistant>
    <dialtone>
    
    </dialtone>
  </turn>
</transcript>
```

This workflow is optimized for LLM agents and operators doing test-driven development (TDD) with strong verification.




# Core principles

### No automatic command runs
The agent/operator runs tests and reports results.

### No automatic git commits
The agent/operator commits after verification.

### `done` is gated
You should only finalize a ticket after:


## Commands you will use
```shell
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket review <ticket-name>
./dialtone.sh ticket subtask list
./dialtone.sh ticket subtask add <name> --desc "..."
./dialtone.sh ticket subtask testcmd <subtask-name> <test-command...>
./dialtone.sh ticket summary update
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
./dialtone.sh ticket done
```

---

## `review` vs `start`

- `./dialtone.sh ticket review <ticket-name>`
  - **Purpose**: prep-only. Use this when you want DIALTONE + the LLM to **review the ticket DB and subtasks** and make sure it’s ready to work on later.
  - **Workflow**: DIALTONE iterates over the ticket and each subtask and asks review questions like:
    1. is the goal aligned with subtasks
    2. should there be more subtasks
    3. are any subtasks too large
    4. is there work that should be put into a different ticket because it is not relevant
    5. does this ticket create a new plugin
    6. does this ticket have a update documentation subtask
    7. does this subtask have the correct test-command
  - **Skips**: does **not** suggest running tests, reviewing logs, or marking subtasks `done`.
  - **Outcome**: the ticket is marked **reviewed** and is ready for `start`.
  - **Repeatable**: while in `review` mode, re-run the review iteration at any time with `./dialtone.sh ticket next`.

- `./dialtone.sh ticket start <ticket-name>`
  - **Purpose**: execution. Use this when you are ready to actually do work on the ticket.
  - **Outcome**: enters the normal subtask/verification workflow.

### Branch rule (applies to `add`, `review`, and `start`)

All ticket work happens on a git branch named **exactly** like the ticket:

- `ticket add <ticket>` / `ticket review <ticket>` / `ticket start <ticket>` should create or switch to the branch named `<ticket>`.

---

## Ticket state

Tickets track a simple `state` field to indicate where they are in the lifecycle:

- `new`: created but not reviewed
- `reviewed`: reviewed and ready to start later
- `started`: execution has begun
- `blocked`: blocked waiting on a question/acknowledgement or missing planning info
- `done`: finalized

---

## Required verification loop (per subtask)

For every subtask, follow this loop until it’s true:

1. **Verify fields**: subtask name/description/test-command are correct.
2. **Run tests**: agent runs the test command and inspects output.
3. **Fix + rerun**: repeat until the test passes.
4. **Review logs**: ensure no ERROR/EXCEPTION and that resources are cleaned up.
5. **Submit summary**: update the subtask summary file and sync it into the ticket DB.
6. **Mark subtask done**: `ticket subtask done <ticket> <subtask>`
7. **Commit**: agent creates a git commit after verification.

### Per-subtask summary files (persistent)

Instead of a single `agent_summary.md`, each ticket uses **one markdown file per subtask**:

- Location: `src/tickets/<ticket-id>/`
- File name: `<subtask-name>-summary.md`

You update the relevant `<subtask-name>-summary.md`, then run:

```shell
./dialtone.sh ticket summary update
```

`ticket summary update` syncs the latest contents into DuckDB so `ticket summary` / `ticket search` work, and **leaves the markdown file in place** (no deletion).

---

## Example transcript: `ticket start`

Command you run:

```shell
./dialtone.sh ticket start www-dev-page-earth-spin
```

What `DIALTONE` says (example):

```shell
DIALTONE:
- ticket: www-dev-page-earth-spin
- goal: keep work ticket-driven; run tests yourself and summarize results
- verify: git branch is correct and working tree is clean before starting

Run the next command(s) to validate environment and begin the first subtask.
Then summarize results and what to do next.

example-commands
./dialtone.sh ticket subtask list
./dialtone.sh plugin test <plugin-name>
./dialtone.sh www dev
```

---

## Example transcript: `ticket next`

Command you run:

```shell
./dialtone.sh ticket next
```

What `DIALTONE` says (example):

```shell
DIALTONE:
- ticket: www-dev-page-earth-spin
- subtask: init
- policy: DIALTONE does not auto-run tests; agent must run and report results
- verify: tests pass; logs contain no ERROR/EXCEPTION; tests clean up resources

Run the subtask test command(s) now.
If it fails, modify code/tests and re-run until it passes. Then review logs and submit a summary.

example-commands
./dialtone.sh ticket subtask list
./dialtone.sh plugin test <plugin-name>
./dialtone.sh logs --lines 200
./dialtone.sh ticket summary update
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
```

---

## Example transcript: marking a subtask done

After you have:
- run tests (pass)
- reviewed logs (clean)
- submitted a summary

Command you run:

```shell
./dialtone.sh ticket subtask done www-dev-page-earth-spin init
```

What `DIALTONE` says (example):

```shell
DIALTONE:
- ticket: www-dev-page-earth-spin
- subtask: init
- record: subtask status marked done (manual verification assumed)
- next: submit subtask summary and prepare a git commit

Please confirm:
- You ran the subtask tests and they passed
- You reviewed logs and found no ERROR/EXCEPTION
- Tests cleaned up any resources they created

Then submit a summary and create a commit.

example-commands
./dialtone.sh ticket summary update
git status -sb
git add .
git commit -m "Describe the change"
./dialtone.sh ticket done
```

---

## Example transcript: `ticket done`

Command you run:

```shell
./dialtone.sh ticket done
```

If a subtask is still incomplete, `DIALTONE` blocks you (example):

```shell
DIALTONE:
- ticket: www-dev-page-earth-spin
- blocker: subtask `init` is still todo
- process: run tests, review logs, submit summary, then mark subtask done

Loop until the subtask test passes and logs are clean.
Then mark the subtask done and re-run ticket done.

example-commands
./dialtone.sh ticket subtask list
./dialtone.sh plugin test <plugin-name>
./dialtone.sh logs --lines 200
./dialtone.sh ticket summary update
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
./dialtone.sh ticket done
```

When all subtasks are done and subtask summary files exist/are up to date, `ticket done`:

- syncs the latest per-subtask summaries into the ticket DB (if needed)
- writes a backup DB named: `src/tickets/<ticket>/<ticket>-backup.duckdb`
- prints the next manual steps (commit/PR)

---

## Notes on log review

The agent/operator should explicitly confirm:

- **No ERROR/EXCEPTION** in browser console logs (Chromedp output) or runtime logs.
- Tests **clean up** ports/processes/files they start.
- Git is in the expected state before starting a ticket or marking it done:

```shell
git status -sb
git diff
```

