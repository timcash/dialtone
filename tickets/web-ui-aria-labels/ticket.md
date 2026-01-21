# Branch: web-ui-aria-labels
# Task: Add ARIA Labels for Accessibility and Automated Testing

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.

## Goals
1. Use tests files in `ticket/web-ui-aria-labels/test/` to drive all work.
2. Add ARIA labels to all interactive elements in the `www` application.
3. Use ARIA labels as primary selectors in `chromedp` automated tests.
4. Improve accessibility for screen readers.

## Non-Goals
1. DO NOT change the visual design or layout.
2. DO NOT introduce new CSS styles unless required for accessibility visibility (e.g., focus states).

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh ticket test web-ui-aria-labels
   ```
2. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Log any accessibility audit warnings or errors during the build process.

## Subtask: Research
- description: Audit current `www` pages for missing ARIA labels and accessibility gaps.
- test: Audit report documented in Collaborative Notes.
- status: todo

## Subtask: Implementation
- description: [MODIFY] `src/plugins/www/app/index.html`: Add `aria-label` to buttons, links, and forms.
- description: [MODIFY] `src/plugins/www/test/e2e_test.go`: Update `chromedp` selectors to use `aria-label`.
- test: E2E tests pass using new ARIA selectors.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Add ARIA labels to all interactive components in the Dialtone `www` web application to improve accessibility and provide robust hooks for automated browser testing via `chromedp`.

## Collaborative Notes
- Use `aria-label` and `role` attributes extensively.
- Ensure all images have descriptive `alt` text.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
