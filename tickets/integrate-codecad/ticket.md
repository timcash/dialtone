# Branch: integrate-codecad
# Task: Integrate CodeCAD into WWW application

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.

## Goals
1. Use tests files in `ticket/integrate-codecad/test/` to drive all work.
2. Integrate the `code_cad` repository (https://github.com/timcash/code_cad) into the `www` application.
3. Display CodeCAD components within the `dialtone-earth` (or current `www`) frontend.
4. Ensure the integration is NOT exposed in the robot's local UI.

## Non-Goals
1. DO NOT integrate into the `src/web/` local UI.
2. DO NOT rewrite CodeCAD; use it as an external dependency or submodule.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh ticket test integrate-codecad
   ```
2. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use logs to track loading of CodeCAD assets in the frontend.

## Subtask: Research
- description: Analyze `code_cad` repository to identify main entry points and exportable components.
- test: Integration approach documented in Collaborative Notes.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/www/app/components/code-cad-viewer.ts`: Create a wrapper component for CodeCAD.
- description: [MODIFY] `src/plugins/www/app/index.html`: Add a section for CodeCAD demonstration.
- test: Manual verification in browser shows CodeCAD rendering correctly.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Integrate the `code_cad` repository into the Dialtone `www` webpage to showcase CAD capabilities within the project's public frontend.

## Collaborative Notes
- CodeCAD Repo: https://github.com/timcash/code_cad
- Ensure the integration uses Vite-compatible import patterns.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
