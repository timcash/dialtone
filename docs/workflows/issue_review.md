---
description: guide for LLM agents to review and prioritize GitHub issues
---

# Workflow: Issue Review & Ticket Preparation

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

### Step 1: List and Scan
```bash
# Audit the backlog to identify p0 and p1 candidates.
./dialtone.sh github issue list --markdown
```

### Step 2: The Decision Loop (p0 -> p1 -> Discretionary)
```bash
# 1. View candidate
./dialtone.sh github issue view <id>

# 2. Decision: IMPROVE
# Requirements are clear. Transform into a ticket.
./dialtone.sh ticket start <name>
# (Populate to Ticket Standard - see Section 3)
./dialtone.sh github issue <id> --ready

# 2. Decision: ASK
# Ambiguous requirements or technical blockers.
./dialtone.sh github issue comment <id> "Need clarification on [X]..."
./dialtone.sh github issue edit <id> --add-label clarification
```

### Step 3: Promotion (Optional)
```bash
# If unprioritized but critical, promote to p0.
./dialtone.sh github issue edit <id> --add-label p0
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
# 2. Add the 'ticket' label to GitHub.
./dialtone.sh github issue <id> --ready
```