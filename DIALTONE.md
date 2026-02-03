# DIALTONE (Virtual Librarian)

`DIALTONE` is a **helpful virtual librarian** embedded in the Dialtone CLI output. It does **not** run tests or git automatically. Instead, after key ticket commands it prints:

- **Context**: what state you’re in and what matters next
- **Next step**: what to do and why
- **Example commands**: copy/paste commands to run manually

This workflow is optimized for LLM agents and operators doing ticket-driven development (TDD) with strong verification.

---

## Core principles

- **No automatic test runs**: the agent/operator runs tests and reports results.
- **No automatic git commits**: the agent/operator commits after verification.
- **`done` is gated**: you should only finalize a ticket after:
  - subtasks are actually complete
  - tests have been run and passed
  - logs have been reviewed and contain no ERROR/EXCEPTION
  - the agent summary has been submitted

---

## Commands you will use

```shell
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket subtask list
./dialtone.sh ticket subtask add <name> --desc "..."
./dialtone.sh ticket subtask testcmd <subtask-name> <test-command...>
./dialtone.sh ticket summary update
./dialtone.sh ticket subtask done <ticket-name> <subtask-name>
./dialtone.sh ticket done
```

---

## Required verification loop (per subtask)

For every subtask, follow this loop until it’s true:

1. **Verify fields**: subtask name/description/test-command are correct.
2. **Run tests**: agent runs the test command and inspects output.
3. **Fix + rerun**: repeat until the test passes.
4. **Review logs**: ensure no ERROR/EXCEPTION and that resources are cleaned up.
5. **Submit summary**: update `agent_summary.md` and run summary ingestion.
6. **Mark subtask done**: `ticket subtask done <ticket> <subtask>`
7. **Commit**: agent creates a git commit after verification.

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
- next: submit agent summary and prepare a git commit

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

When all subtasks are done and `agent_summary.md` exists, `ticket done`:

- ingests the summary into the ticket DB
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

