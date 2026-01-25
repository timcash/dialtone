# Branch: geospatial-tools
# Task: Integrate USGS LIDAR tools for point cloud processing

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create lidar` to create the new plugin structure.

## Goals
1. Use tests files in `ticket/geospatial-tools/test/` to drive all work.
2. Create a `lidar` plugin in `src/plugins/lidar/`.
3. Integrate tools from `https://github.com/opengeos/maplibre-gl-usgs-lidar` for processing LIDAR data.
4. Support visualization of point clouds in the `www` application.

## Non-Goals
1. DO NOT implement a full 3D engine; use existing MapLibre-based tools.
2. DO NOT download large datasets during tests; use minimal samples.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh test ticket geospatial-tools
   ```
2. **Plugin Tests**: Run its specific tests.
   ```bash
   ./dialtone.sh test plugin lidar
   ```
3. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use the `src/logger.go` package to log messages.

## Subtask: Research
- description: Analyze `maplibre-gl-usgs-lidar` for integration points into the Dialtone Go/TS stack.
- test: Integration plan documented in Collaborative Notes.
- status: todo

## Subtask: Scaffold
- description: Run `./dialtone.sh plugin create lidar` and verify structure.
- test: `src/plugins/lidar/` exists.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/lidar/app/lidar_worker.ts`: Implement data processing worker.
- description: [MODIFY] `src/plugins/www/app/index.html`: Add a LIDAR visualization layer.
- test: Point cloud sample renders in the browser during manual verification.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Integrate tools from the `maplibre-gl-usgs-lidar` point cloud library to support LIDAR data processing and visualization within the Dialtone ecosystem.

## Collaborative Notes
- Use `GH_TOKEN` for automated GitHub interactions.
- Ensure the sub-agent is restricted to a specific workspace.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`

