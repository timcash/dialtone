# Branch: social-media-plugin
# Task: Social Media Interaction Plugin

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create social` to create the new plugin structure.

## Goals
1. Use tests files in `ticket/social-media-plugin/test/` to drive all work.
2. Create a `social` plugin in `src/plugins/social/`.
3. Support basic interaction (search, post, reply) with Facebook, X, Instagram, Slack, and YouTube.
4. Focus on championing the Dialtone project to developers and researchers.

## Non-Goals
1. DO NOT implement complex bot behavior that violates platform terms of service.
2. DO NOT store user credentials in plain text.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh test ticket social-media-plugin
   ```
2. **Plugin Tests**: Run its specific tests.
   ```bash
   ./dialtone.sh test plugin social
   ```
3. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use logs to track API requests and responses (scrubbing sensitive data).

## Subtask: Research
- description: Review APIs for X (Twitter), Slack, and YouTube to identify post/search integration points.
- test: API capability matrix documented in Collaborative Notes.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/social/app/slack_client.go`: Implement basic Slack posting logic.
- description: [NEW] `src/plugins/social/cli/social.go`: Add `dialtone.sh social post --platform slack` command.
- test: Integration test verifies message is sent to a test Slack channel.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Create a plugin to allow Dialtone to interact with social media feeds. The goal is to efficiently search, explore, reply, and post to help champion the project across various online communities.

## Collaborative Notes
- Focus on Slack and X as the first priority platforms.
- Use the standard plugin registration pattern.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
