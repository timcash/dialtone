# DuckDB Ticket Workflow V2 (LLM Agent Edition)

This document drafts the "LLM Agent First" workflow for Dialtone. In this version, the CLI doesn't just execute commands; it acts as a collaborator by providing alerts, capturing context, and managing agent-specific resources like temporary API keys.

## 1. Contextual Start
The agent starts a ticket. The CLI responds with the current schema state and any "lingering" context from previous related tickets.

```bash
./dialtone.sh ticket start feature-api-integration
```

**Agent Perspective:**
> "I am starting the API integration ticket. Looking for existing patterns."

**CLI Response:**
```text
[ticket] Started: feature-api-integration
[ALERT] Similar patterns found in 'previous-stripe-plugin'. 
[CONTEXT] Found 2 related subtasks in DuckDB archives. Would you like to import them?
```

## 2. Conversational Ambiguity (The "Email" Loop)
The agent asks a question. Later, when the agent tries to move forward, the CLI blocks the action because a "response" (human or system) is pending or has just arrived.

```bash
./dialtone.sh ticket ask "Which API version should we target for the new plugin?"
```

**Later, the agent tries to progress:**
```bash
./dialtone.sh ticket next
```

**CLI Response:**
```text
[BLOCK] Cannot proceed to 'next' subtask.
[MESSAGE] User 'dev' answered your question: "Use v3.2 of the REST API."
[ACTION] Please acknowledge this message to continue: ./dialtone.sh ticket ack
```

## 3. Dynamic Resource Management (Future)
The agent needs to run a test that requires a 3rd party API. `dialtone.sh` manages a short-lived token.

```bash
./dialtone.sh ticket grant --scope "stripe-sandbox" --duration "10m"
```

**CLI Response:**
```text
[AUTH] Temporary API key granted for 10 minutes.
[ENV] STRIPE_API_KEY exported to current session.
[WARN] Key will automatically expire at 09:15:00.
```

## 4. The "Alert" during Execution
While the agent is working on a subtask, an external event (like a failing CI job or a design doc update) triggers an alert during the `log` or `status` commands.

```bash
./dialtone.sh ticket log "Implementing the webhook handler."
```

**CLI Response:**
```text
[ticket] Log captured.
[ALERT] Design Doc 'api_spec.md' was modified 2 minutes ago. 
[DIFF] Field 'webhook_url' renamed to 'callback_uri'. 
Please update your implementation.
```

## 5. Final Validation with Conversational Summary
Instead of just "Valid", the CLI asks for a summary of "Lessons Learned" to store in DuckDB for future agents.

```bash
./dialtone.sh ticket done
```

**CLI Response:**
```text
[ticket] Validating... Pass.
[PROMPT] What is one tip for the next agent working on this plugin?
Usage: ./dialtone.sh ticket done --tip "Remember to escape the webhook secret."
```
