# Collaboration Log Structure

The collaboration log records the interaction between the User, AI Agents, and the Dialtone system. It serves as a linear, auditable history of the task execution.

## Format Specification

### 1. Header
Initializes the session context.
```text
DIALTONE> Loading Task DAG for [task-id]...
DIALTONE> Current State: <state description>
DIALTONE> Assigned: <List of Roles>
```

### 2. Dialogue & Actions
Standard interaction format.
```text
<ACTOR>> <Message or Action>
```
**Actors**: `USER-n`, `LLM-CODE`, `LLM-TEST`, `LLM-REVIEW`, `DIALTONE`, `DIALTONE:<PID>` (Subtone)
**Actions**:
- **Chat**: Plain text message.
- **Edit**: `[Edit <file>: <description>]`
- **Command**: `@DIALTONE <command>`

### Subtones (Process Execution)
When a command is confirmed, a "subtone" comes online to run that command.
- **ID**: `DIALTONE:<PID>>` (where `N` is the `PID`).
- **Parent**: The process that ran `./dialtone.sh`.
- **Function**: Runs the command in the background (as a child process) and streams output.

### 3. Consensus Mechanism (Signatures)
Required for executing commands or advancing tasks.
```text
DIALTONE> Request received. Sign with `@DIALTONE task --sign <task-id>`...
<ACTOR>> @DIALTONE task --sign <task-id> [--blocked]
DIALTONE> Signatures verified. <Action>...
```

### 4. Task Completion
Protocol for finishing a task.
1.  **Verification**: Tests pass.
2.  **Review**: Reviewer signs off.
3.  **Final Signatures**: All actors sign.
4.  **Summary & Scoring**: Report generation and quality scoring.

```text
DIALTONE> All required signatures present. Task marked as DONE.
DIALTONE> Summary written to docs/reports/<task-id>.md
<ACTOR>> Scores: criteria=X/5, outcomes=Y/5, notes=<text>
```

### 5. Example Sequence
Demonstrates a command spawning a process (Subtone).

```text
USER-1> @DIALTONE npm run test
DIALTONE> Request received. Sign with `@DIALTONE task --sign test-task`...
USER-1> @DIALTONE task --sign test-task
DIALTONE> Signatures verified. Running command via PID 4512...

DIALTONE:4512> > dialtone@1.0.0 test
DIALTONE:4512> > tap "test/*.js"
DIALTONE:4512> 
DIALTONE:4512> [PASS] test/basic.js
DIALTONE:4512> Tests completed successfully.

DIALTONE:4512> [EXIT] Process exited with code 0.
```

### 6. Example: Error Investigation
Demonstrates detailed error analysis using standard tools.

```text
USER-1> @DIALTONE npm run build
DIALTONE> Request received. Sign with `@DIALTONE task --sign build-task`...
USER-1> @DIALTONE task --sign build-task
DIALTONE> Signatures verified. Running command via PID 5102...

DIALTONE:5102> [ERROR] Build failed.
DIALTONE:5102> [ERROR] count:14 types:3
DIALTONE:5102> [EXIT] Process exited with code 1.

LLM-CODE> @DIALTONE "grep -C 2 'ERROR' build.log | sort | uniq -c"
DIALTONE> Request received. Sign with `@DIALTONE task --sign log-analysis`...
USER-1> @DIALTONE task --sign log-analysis
DIALTONE> Signatures verified. Running command via PID 5108...

DIALTONE:5108>    4 [ERROR] TS2322: Type 'string' is not assignable to type 'number'.
DIALTONE:5108>    2 [ERROR] Module not found: Error: Can't resolve './utils'
DIALTONE:5108>    8 [ERROR] Unexpected token
DIALTONE:5108> [EXIT] Process exited with code 0.
```

### 7. Full Workflow: Auth Middleware Refactor
A complex session involving multiple users, agents, subtasks, and a deployment verification.

![Task Flow](file:///c:/Users/timca/code3/dialtone/src/plugins/www/TASKS.md)

**Context**: Task `auth-middleware-v2` has been started.

```text
USER-1> @DIALTONE task start auth-middleware-v2
DIALTONE> Loading Task DAG for [auth-middleware-v2]...
DIALTONE> Loading subtasks: [auth-docs-update], [auth-tests-fix], [auth-deployment-verify]...
DIALTONE> Current State: 3 Pending Subtasks.
DIALTONE> Assigned: USER-1 (Lead), LLM-CODE (Impl), LLM-TEST (QA), LLM-REVIEW (Audit).

USER-1> Let's start with the docs update while code runs.
LLM-CODE> I'll handle the middleware refactor.
LLM-CODE> [Edit src/auth/middleware.js: Implement JWT validation]

USER-2> I'll review the new API spec.
USER-2> @DIALTONE task start auth-docs-update
DIALTONE> Switching context to [auth-docs-update].

LLM-CODE> @DIALTONE npm run test:auth
DIALTONE> Request received. Sign with `@DIALTONE task --sign auth-middleware-v2`...
USER-1> @DIALTONE task --sign auth-middleware-v2
LLM-TEST> @DIALTONE task --sign auth-middleware-v2
DIALTONE> Signatures verified. Running tests via PID 8821...

DIALTONE:8821> > dialtone-auth@2.0.0 test:auth
DIALTONE:8821> > tap "src/auth/tests/*.js"
DIALTONE:8821> 
DIALTONE:8821> [FAIL] src/auth/tests/login_test.js
DIALTONE:8821>    Error: Token mismatch. Expected JWT, got Basic.
DIALTONE:8821> [FAIL] src/auth/tests/refresh_token_test.js
DIALTONE:8821>    Error: Timeout waiting for refresh.
DIALTONE:8821> 
DIALTONE:8821> [EXIT] Process exited with code 1.

LLM-TEST> We have regressions. Starting `auth-tests-fix`.
LLM-TEST> @DIALTONE task start auth-tests-fix
DIALTONE> Switching context to [auth-tests-fix].

LLM-CODE> I see the issue. The middleware mock wasn't updated in the test fixtures.
LLM-CODE> [Edit src/auth/tests/fixtures.js: Update mock to return JWT]
LLM-CODE> @DIALTONE npm run test:auth:flaky
DIALTONE> Request received. Sign with `@DIALTONE task --sign auth-tests-fix`...
USER-1> @DIALTONE task --sign auth-tests-fix
LLM-TEST> @DIALTONE task --sign auth-tests-fix
DIALTONE> Signatures verified. Running tests via PID 8890...

DIALTONE:8890> > dialtone-auth@2.0.0 test:auth:flaky
DIALTONE:8890> [PASS] src/auth/tests/login_test.js
DIALTONE:8890> [PASS] src/auth/tests/refresh_token_test.js
DIALTONE:8890> [EXIT] Process exited with code 0.

LLM-TEST> Fix verified. Marking `auth-tests-fix` as DONE.
LLM-TEST> @DIALTONE task finish auth-tests-fix
DIALTONE> Task [auth-tests-fix] marked as DONE. Resuming parent [auth-middleware-v2].

USER-2> API docs are updated. `auth-docs-update` is ready.
USER-2> @DIALTONE task finish auth-docs-update
DIALTONE> Task [auth-docs-update] marked as DONE.

USER-1> All pre-reqs done. Deploying to verify.
USER-1> @DIALTONE task start auth-deployment-verify
DIALTONE> Switching context to [auth-deployment-verify].

LLM-CODE> @DIALTONE npm run verify:staging
DIALTONE> Request received. Sign with `@DIALTONE task --sign auth-deployment-verify`...
USER-1> @DIALTONE task --sign auth-deployment-verify
USER-2> @DIALTONE task --sign auth-deployment-verify
LLM-REVIEW> @DIALTONE task --sign auth-deployment-verify
DIALTONE> Signatures verified. Running deployment verification via PID 9012...

DIALTONE:9012> [INFO] Deploying to staging...
DIALTONE:9012> [INFO] Service updated.
DIALTONE:9012> [INFO] Running health checks...
DIALTONE:9012> [PASS] GET /health (200 OK)
DIALTONE:9012> [PASS] OAuth exchange flow
DIALTONE:9012> [EXIT] Process exited with code 0.

LLM-REVIEW> Verification looks solid.
LLM-REVIEW> @DIALTONE task finish auth-deployment-verify
DIALTONE> Task [auth-deployment-verify] marked as DONE.

DIALTONE> Parent task [auth-middleware-v2] is now ready for completion.
DIALTONE> Generating report...
DIALTONE> Summary written to docs/reports/auth-middleware-v2.md
DIALTONE> Please update AGENT.md with learnings.

LLM-CODE> [Edit src/plugins/www/AGENT.md: Add notes on token refresh race condition]
LLM-CODE> Updated AGENT.md.

USER-1> @DIALTONE task finish auth-middleware-v2
DIALTONE> All tasks completed.
```
