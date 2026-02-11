---
trigger: model_decision
description: example workflow for ticket review mode
---

# Workflow: Ticket Review (Example)

This walkthrough shows how `ticket review` guides planning and field-by-field verification without suggesting tests, logs, or marking subtasks done.

## 1) Create the ticket and enter review mode

```shell
./dialtone.sh ticket review fake-webpage-update
```

### Example DIALTONE output (excerpt)
```text
DIALTONE:
- ticket: fake-webpage-update
- mode: review (prep-only)
- policy: do not demand tests, logs, or code changes
- goal: ensure the ticket DB/subtasks are ready for `ticket start` later
- verify: branch name matches ticket name
- validate: ticket DB/subtasks loaded successfully
- state: review

Review questions (ticket + each subtask):
1. is the goal aligned with subtasks
2. should there be more subtasks
3. are any subtasks too large
4. is there work that should be put into a different ticket because it is not relevant
5. does this ticket create a new plugin
6. does this ticket have a update documentation subtask
7. does this subtask have the correct test-command

Notes:
- review mode skips suggesting tests/log review or marking subtasks done
- summary files exist at `src/tickets/<ticket>/<subtask>-summary.md` (created if missing)
```

## 2) Review ticket fields and improve subtasks

Use `ticket review` output to check every ticket/subtask field and fill missing values.

### Add or refine subtasks
```shell
./dialtone.sh ticket subtask add update-hero --desc "Refresh hero copy and CTA"
./dialtone.sh ticket subtask add update-layout --desc "Tweak layout spacing for new hero"
./dialtone.sh ticket subtask add update-images --desc "Swap hero image assets"
./dialtone.sh ticket subtask add update-docs --desc "Document the new webpage update"
```

### Add test commands per subtask
```shell
./dialtone.sh ticket subtask testcmd update-hero ./dialtone.sh www test
./dialtone.sh ticket subtask testcmd update-layout ./dialtone.sh www test
./dialtone.sh ticket subtask testcmd update-images ./dialtone.sh www test
./dialtone.sh ticket subtask testcmd update-docs "grep -n \"fake-webpage-update\" docs/ -R"
```

## 3) Re-run review iteration

```shell
./dialtone.sh ticket next
```

### Example DIALTONE output (excerpt)
```text
[REVIEW] Field checks (ticket):
- id: fake-webpage-update -> is this correct?
- name: fake-webpage-update -> is this correct?
- tags: (empty) -> is this correct?
- description: (empty) -> is this correct?
- state: review -> is this correct?
...

[REVIEW] Field checks (subtask):
- name: update-hero -> is this correct?
- description: Refresh hero copy and CTA -> is this correct?
- test-command: ./dialtone.sh www test -> is this correct?
...
```

## 4) Mark reviewed once everything is correct

```shell
./dialtone.sh ticket reviewed fake-webpage-update
```

At this point the ticket `state` is `reviewed`, all subtasks have a `reviewed_timestamp`, and the ticket is ready for `ticket start` when execution begins.
