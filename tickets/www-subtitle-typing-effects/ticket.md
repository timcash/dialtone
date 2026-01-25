# Branch: www-subtitle-typing-effects
# Task: Subtitle Typing Effects and WWW Branding

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.

## Goals
1. Use tests files in `ticket/www-subtitle-typing-effects/test/` to drive all work.
2. Formally rename `dialtone-earth` references to `www` in the CLI and repository.
3. Implement a typing effect for the main page subtitle.
4. Cycle through 3 scifi-themed subtitles related to unified robotic networks.

## Non-Goals
1. DO NOT change the core styling of the page beyond the typing effect.
2. DO NOT introduce heavy external animation libraries if simple CSS/JS suffices.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh test ticket www-subtitle-typing-effects
   ```
2. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Log CLI command execution for the new `www` branding.

## Subtask: Research
- description: Review `src/dev.go` for all `dialtone-earth` string references.
- test: List of files needing renaming documented in Collaborative Notes.
- status: todo

## Subtask: Implementation
- description: [MODIFY] Global rename: `dialtone-earth` -> `www` in relevant CLI code and directories.
- description: [NEW] `src/plugins/www/app/typing.js`: Implement the typing and cycling logic.
- description: [MODIFY] `src/plugins/www/app/index.html`: Integrate the typing effect script.
- test: Manual verification shows subtitles cycling with a typing animation.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Rename the public-facing application to `www` for clarity and add engaging typing effects to the main landing page subtitles to match the project's scifi aesthetic.

## Collaborative Notes
- Subtitles to cycle: "Unified Robotic Networks", "Autonomous Agent Collaboration", "The Future of Distributed Intelligence".
- Ensure the renaming doesn't break existing Vercel deployment configurations.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
