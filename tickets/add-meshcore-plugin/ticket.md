# Branch: add-meshcore-plugin
# Task: Add MeshCore plugin for remote management

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create meshcore` to create the new plugin structure.

## Goals
1. Use tests files in `ticket/add-meshcore-plugin/test/` to drive all work.
2. Create a new `meshcore` plugin in `src/plugins/meshcore/`.
3. Implement basic MeshCore agent communication for remote management.
4. Support secure registration and command execution from a MeshCentral server.

## Non-Goals
1. DO NOT implement a full MeshCentral server; focus on the agent plugin.
2. DO NOT bypass existing security models.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh ticket test add-meshcore-plugin
   ```
2. **Plugin Tests**: Run its specific tests.
   ```bash
   ./dialtone.sh plugin test meshcore
   ```
3. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use the `src/logger.go` package to log messages.

## Subtask: Research
- description: Review MeshCore agent communication protocols and Go implementations.
- test: Documentation in Collaborative Notes about registration flow.
- status: todo

## Subtask: Scaffold
- description: Run `./dialtone.sh plugin create meshcore` and verify structure.
- test: `src/plugins/meshcore/` exists.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/meshcore/app/agent.go`: Implement the MeshCore agent loop.
- test: Integration test simulates a server handshake and verifies response.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Implement a MeshCore plugin to enable remote management and monitoring of Dialtone-enabled devices.

## Collaborative Notes
- Ensure compatibility with standard MeshCentral servers.
- Use the `core/logger` for all internal agent logging.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`

