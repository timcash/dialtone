# Ticket Analysis

## Top 5 Best Tickets
These tickets exemplify the standards set in `docs/ticket-template.md`. They have clear goals, properly formatted subtasks, and verifiable test commands.

1. **[issue-ticket-workflow](good/issue-ticket-workflow/ticket.md)**
   - **Why it's good**: Complete adherence to the template. Detailed goal connecting to business value ("streamlined triage process"). Subtasks are granular with precise `test-command` verification steps (e.g., `grep` checks for documentation updates). Statuses are correctly tracked.

2. **[cloudflare-tunnel](good/cloudflare-tunnel/ticket.md)**
   - **Why it's good**: Excellent breakdown of a complex feature (wrapping an external binary). Subtasks cover the entire lifecycle: install, login, tunnel management, cleanup. Test commands are specific and cover edge cases (e.g., idempotency, cleanup).

3. **[mavlink-6dof](good/mavlink-6dof/ticket.md)**
   - **Why it's good**: Clearly defines the integration of a specific protocol (MAVLink with 6DOF). Subtasks span extraction, transport, and UI display. Includes subtasks for reliability improvements (`chromedp-reliability`) and remote verification, showing a complete definition of done.

4. **[ai-migration](good/ai-migration/ticket.md)**
   - **Why it's good**: Strong example of a refactoring/migration ticket. It breaks down the movement of logic from core files to a plugin structure. Test commands verify both the existence of new files (`cat ...`) and the functionality (`./dialtone.sh ai build`).

5. **[three-column-3d-ui](good/three-column-3d-ui/ticket.md)**
   - **Why it's good**: Extremely thorough UI ticket. It breaks down visual changes into checkable backend and frontend components. Includes integration tests, remote deployment verification, and even diagnostics updates.

## 5 Tickets Needing Improvement
These tickets fail to meet the standards, often containing unfilled placeholders or non-standard structures.

1. **[fake-ticket](bad/fake-ticket/ticket.md)**
   - **Issues**: Deviates completely from the standard template. Uses `test-condition-1`/`test-condition-2` instead of `test-command`. Uses `pass-timestamp` which is not in the standard. This structure breaks tooling that expects the standard format.

2. **[ticket-start-final-check](bad/ticket-start-final-check/ticket.md)**
   - **Issues**: Contains raw template placeholders (`<subtask-title>`, `<subtask-name>`, `<goal>`) that were never filled in. This ticket is essentially empty and non-actionable.

3. **[test-automation-verify](bad/test-automation-verify/ticket.md)**
   - **Issues**: Similar to the above, it retains `<subtask-title>` and `<subtask-name>` placeholders. It shows a failure to properly scope the work before creating the ticket.

4. **[ticket-validation-command](bad/ticket-validation-command/ticket.md)**
   - **Issues**: While it has subtasks, it leaves the top-level metadata empty (`Tags: <tags>`, `# Goal <goal>`). A ticket without a clear goal definition relies on tribal knowledge.

5. **[gemini-cli-integration](bad/gemini-cli-integration/ticket.md)**
   - **Issues**: Contains actual work and subtasks, but failed to fill in the header information (`Tags: <tags>`, `# Goal <goal>`). Even though the work is done, the documentation aspect of the ticket is incomplete.
