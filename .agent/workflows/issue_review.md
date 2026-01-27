---
trigger: model_decision
description: github issue triage workflow for LLM agents
---

# Workflow: Issue Review & Ticket Preparation

> [!IMPORTANT]
> This is a **planning and question** workflow designed to triage and prepare issues. It is **NOT** an execution workflow. Do **NOT** write implementation code while following this process.

This is the **Source of Truth** for the Dialtone Issue Management API.

## 1. CLI API Reference
```bash
# List open issues in a clean, agent-readable markdown table.
./dialtone.sh github issue list --markdown

# Shortcut: Mark an issue as 'ticket' ready (adds the label).
./dialtone.sh github issue <id> --ready

# View complete details, labels, and comments for an issue.
./dialtone.sh github issue view <id>

# Add a specific label (e.g., p0, p1, ready).
./dialtone.sh github issue edit <id> --add-label <name>

# Add a clarifying question or update to an issue.
./dialtone.sh github issue comment <id> "<message>"
```

---

## 2. Automated Triage Workflow

### Step 1: Backlog Audit
Scan all open issues to identify high-priority candidates.
```bash
./dialtone.sh github issue list --markdown
```

### Step 2: Issue Deep-Dive
Inspect the specific goals and constraints of a candidate issue.
```bash
./dialtone.sh github issue view <id>
```

### Step 3: Priority Alignment
Categorize the issue immediately using shortcut flags.
```bash
# If critical, promote to p0.
./dialtone.sh github issue <id> --p0

# If important but not urgent, mark as p1.
./dialtone.sh github issue <id> --p1
```

### Step 4: Resolution: ASK
If the goal is ambiguous or technical blockers exist, request clarification.
```bash
./dialtone.sh github issue comment <id> "Need clarification on [X]..."
./dialtone.sh github issue <id> --question

# Example (Issue #96: "memory patterns"):
#   "Could you clarify which specific memory patterns should be prioritized?"
```

### Step 5: Resolution: IMPROVE
If requirements are clear but lack subtasks, scaffold a local ticket.
```bash
# Scaffold local ticket WITHOUT switching branches.
./dialtone.sh ticket add <name>

# Example (Issue #104: "improve install"):
#   Scaffold 'improve-install-plugin' and populate with atomic subtasks.
```

### Step 6: Readiness Validation
Verify the [Ticket Standard](#3-the-ticket-standard) and mark as ready.
```bash
# 1. Verify all subtasks and tests are defined in the local ticket.
# 2. Mark the GitHub issue as 'ready' and 'ticket' using the shortcuts.
./dialtone.sh github issue <id> --ready --ticket
```

---

## 3. The "Ticket" Standard

An issue is **Ticket Ready** ONLY when a local `ticket.md` meets these criteria:

```bash
# 1. STRUCTURED GOAL: Clear, high-level objective.
# 2. ATOMIC SUBTASKS: Single logic changes (< 30 mins).
# 3. TEST DEFINITIONS: Every subtask has a specific 'test-command'.
# 4. CONTEXTUAL MATERIAL: Links to relevant files and implementation notes.
```

## 4. Finalizing
```bash
# 1. Verify all subtasks and tests are defined in the local ticket.
# 2. Mark the GitHub issue as 'ready' and 'ticket' using the shortcuts.
./dialtone.sh github issue <id> --ready --ticket
```

---

## 5. Labeling Reference

Use these flags with `github issue <id> --<flag>` to categorize issues.

| Flag | Label | Description |
| :--- | :--- | :--- |
| `--p0` | `p0` | Urgent and important. |
| `--p1` | `p1` | Important but NOT urgent. |
| `--bug` | `bug` | Feature not working correctly. |
| `--ready` | `ready` | Ready to be worked on. |
| `--ticket` | `ticket` | Validated for a ticket. |
| `--enhancement` | `enhancement` | New feature or upgrade. |
| `--docs` | `documentation` | Documentation task. |
| `--perf` | `performance` | Performance improvement. |
| `--security` | `security` | Security improvement. |
| `--refactor` | `refactor` | Code structure improvement. |
| `--test` | `test` | Test coverage improvement. |
| `--duplicate` | `duplicate` | Already exists. |
| `--wontfix` | `wontfix` | Not going to fix. |
| `--question` | `question` | Needs clarification. |