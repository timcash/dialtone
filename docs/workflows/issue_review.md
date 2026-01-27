---
description: guide for LLM agents to review and prioritize GitHub issues
---

# Workflow: Issue Review & Ticket Preparation

This workflow guides LLM agents through the process of auditing GitHub issues, prioritizing them, and transforming them into "Ready" tickets.

## 1. List and Scan
Start by listing all open issues with their current tags/labels.

```bash
./dialtone.sh github issue list --markdown
```

## 2. Prioritized Review Loop

Repeat this process for each priority level, starting with **p0**, then **p1**, and so on.

### A. Triage p0 Issues
Focus on these first. They are the most critical.
1. **View Details**:
   ```bash
   ./dialtone.sh github issue view <id>
   ```
2. **Decision: Improve or Ask?**:
   - **Improve**: If the requirements are actionable but the ticket lacks subtasks or tests, **Improve it**. Update the synced ticket to meet the "Ready" standard.
   - **Ask**: If the goal is ambiguous, the technical path is unknown, or requirements are missing, **Ask a clarifying question**.
     ```bash
     ./dialtone.sh github issue comment <id> "I'm triaging this for development. I'm currently missing [X] to move this to a 'Ready' state. Could you clarify [Question]?"
     ```
   - **Ready**: if it's already "Ready", move on.

### B. Triage p1 Issues
Once p0s are triaged (either improved, commented upon, or already Ready), move down to the p1 set and repeat the decision loop.

### C. Continuous Processing
Continue working through the list until you have:
- Marked issues with a `ready` tag (via the collaborative session or by syncing and validating).
- Or run out of time for the current task session.

## 3. The "Ready" Standard
An issue is considered **Ready** when it has been transformed into a ticket that an autonomous agent can pick up and execute without further clarification.

A "Ready" ticket must meet these criteria:
1. **Core Ticket Format**: Uses the standard `ticket.md` structure (Goal, Subtasks, etc.).
2. **Small Actionable Subtasks**: Each subtask should represent a small logic change (ideally < 30 mins of work).
3. **Test Suggestions**: Every subtask has specific, actionable test suggestions or `test-command` definitions.
4. **Validated Material**: All necessary context, files to modify, and specific implementation details are documented in the ticket.

## 4. Finalizing
Once an issue is fully prepared and synced:
1. Update the GitHub issue to reflect it is ready for work.
2. Label it as `ready`.