# Branch: add-geospatial-plugin
# Task: Add core geospatial plugin

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh plugin create geospatial` to create the new plugin structure.

## Goals
1. Use tests files in `ticket/add-geospatial-plugin/test/` to drive all work.
2. Create a core `geospatial` plugin in `src/plugins/geospatial/`.
3. Support basic GeoJSON parsing and spatial operations (e.g., bounding box check, distance calculation).
4. Integrate with the existing `logger` package.
5. Provide a CLI command `dialtone.sh geospatial` for basic utility operations.

## Non-Goals
1. DO NOT implement complex GIS server functionality.
2. DO NOT use external heavy dependencies if light Go libraries (like `orb` or `go-geom`) suffice.

## Test
1. **Ticket Tests**: Run tests specific to this ticket's implementation.
   ```bash
   ./dialtone.sh test ticket add-geospatial-plugin
   ```
2. **Plugin Tests**: Run its specific tests.
   ```bash
   ./dialtone.sh test plugin geospatial
   ```
3. **All Tests**: Run the entire test suite.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use the `src/logger.go` package to log messages.

## Subtask: Research
- description: Research Go-based geospatial libraries (e.g., `github.com/paulmach/orb`).
- test: Decision log in Collaborative Notes on why a specific library was chosen.
- status: todo

## Subtask: Scaffold
- description: Run `./dialtone.sh plugin create geospatial` and verify structure.
- test: `src/plugins/geospatial/` exists and contains `app`, `cli`, and `test` directories.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/geospatial/app/geojson.go`: Implement basic GeoJSON Parsing.
- test: Integration test verifies parsing of a simple FeatureCollection.
- status: todo

## Subtask: Implementation
- description: [NEW] `src/plugins/geospatial/cli/geospatial.go`: Add `dialtone.sh geospatial distance` command.
- test: CLI command returns correct distance between two points.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: todo

## Issue Summary
Implement a core geospatial plugin to provide common spatial utilities to the Dialtone project.

## Collaborative Notes
- Focus on `orb` as it is lightweight and performant.
- Ensure the plugin follows the established patterns in `src/plugins/install`.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`

