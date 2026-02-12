# src_v3 Implementation Guide (Agent Notes)

**Greenfield Notice:** `src/libs/ui_v2` and `src/libs/test_v2` are clean-slate libraries. Avoid copying idiosyncratic behaviors or legacy quirks from `src/libs/ui` or `src/libs/dialtest`. The goal is a simpler, more testable design that removes vestigial patterns.

This guide describes how to build `src_v3` bottom-up, run commands, and validate one test at a time.

## Goals Recap
- New template version in `src/plugins/template/src_v3`.
- Use `src/libs/ui_v2` and `src/libs/test_v2` only.
- No animated transitions between sections.
- Use ARIA labels for all test selectors.
- Build a full test pipeline that generates `TEST.md`, `test.log`, and `error.log`.

## Reference Docs to Consult
- `src/plugins/template/README.md` (capabilities to preserve)
- `src/plugins/template/src_v2` (legacy implementation reference)
- `src/libs/ui` and `src/libs/dialtest` (legacy behaviors to understand, not copy)
- `src/plugins/template/src_v3/DESIGN.md`
- `src/plugins/template/src_v3/TEST_EXAMPLE.md` (format guide, not rigid)

## What to Avoid Copying From v2
- Complex or implicit section transitions (fades, delays, animations).
- Non-ARIA selectors or fragile DOM class selectors in tests.
- Long preflight chains that block tests when UI build stalls.
- UI styles that require custom classes instead of semantic HTML.
- Logs or invariants that are hard to parse or inconsistent across sections.

## Recommended Workflow (Bottom-Up)
Work in small increments and only move forward when the current step passes.

1. **Skeleton layout**
   - Copy `src_v2` into `src_v3` as a baseline.
   - Remove old imports to `src/libs/ui` and `src/libs/dialtest`.
   - Wire `src/libs/ui_v2` and `src/libs/test_v2`.

2. **UI v2 library first**
   - Implement the minimal UI utilities in `src/libs/ui_v2`:
     - Section manager with fast swaps.
     - Menu with modal.
     - Header/menu visibility toggles.
     - Lifecycle logs.
   - Add a base CSS that styles common elements (`section`, `header`, `button`, etc.).

3. **Minimal test runner**
   - Implement `src/libs/test_v2/test.go` with:
     - Preflight (fmt, vet, build, lint, format, build UI).
     - Server start + heartbeat.
     - Chromedp attach + console tap.
     - Logging to stdout, `test.log`, and `error.log`.
   - Add a single “hello world” test step in `src_v3/test` that only checks the hero section.

4. **Scale UI sections one at a time**
   - Add sections in order:
     1) `hero`
     2) `docs`
     3) `table`
     4) `three`
     5) `xterm`
     6) `video`
   - Each section must have:
     - A unique `aria-label` for the section container.
     - `aria-label`s for key controls.
     - Fast section swap behavior.
     - A screenshot captured in tests.

5. **Expand tests incrementally**
   - Add one section test at a time.
   - Ensure the test runner appends to `TEST.md` and `test.log` at each step.
   - Verify `error.log` only captures errors or non-zero exits.

## Commands (Local)
Run commands directly from repo root:

```bash
# Build UI assets
./dialtone.sh template build src_v3

# Run tests (bottom-up)
./dialtone.sh template test src_v3

# Run dev server (debug + chromedp)
./dialtone.sh template dev src_v3
```

## How to Work One Test at a Time
- Only add a new test after the previous one passes.
- Keep the test sequence minimal and deterministic.
- For each test:
  - Add ARIA labels.
  - Add a single test step.
  - Run `./dialtone.sh template test src_v3`.
  - Confirm `TEST.md` updated and the screenshot was created.
- Use `src/plugins/template/src_v3/TEST_EXAMPLE.md` as a guide for report structure, but treat it as flexible and adjust as needed.

## Dev Server + Tests Together
- Keep dev server running in debug mode.
- Ensure tests refresh/rebuild the UI so the dev server reflects latest code.
- Stream console logs to stdout and `dev.log` in `src_v3`.

## Success Criteria
- All sections render without transitions.
- Tests only use ARIA selectors.
- `TEST.md`, `test.log`, and `error.log` are correct and complete.
- Cleanup confirms all Chrome processes are terminated.
