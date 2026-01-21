# Branch: cross-build-arm
# Task: Cross-compile for Linux ARM using Podman

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests in `tickets/cross-build-arm/test/` to drive all work.
2. Migrate the `build` command from `src/dev.go` and `src/build.go` into its own plugin at `src/plugins/build/`.
3. Support `dialtone.sh build --podman --linux-arm` for 32-bit ARM (Raspberry Pi Zero/3/4/5).
4. Support `dialtone.sh build --podman --linux-arm64` for 64-bit ARM (Raspberry Pi 3/4/5).
5. Extend `dialtone.sh install` at `src/plugins/install/cli/install.go` to support necessary build tools (cross-compilers and Podman).
6. Ensure `CGO_ENABLED=1` and correct cross-compilers are used in Podman (`gcc-arm-linux-gnueabihf` for 32-bit, `gcc-aarch64-linux-gnu` for 64-bit).
7. Verify build command works from Windows WSL.

## Non-Goals
1. DO NOT change native build behavior.
2. DO NOT introduce new Docker/Podman images if `golang:1.25.5` is sufficient.
3. DO NOT implement remote flashing in this ticket.

## Test
1. **Ticket Tests**: Run tests specific to the ARM cross-compilation logic.
   ```bash
   ./dialtone.sh ticket test cross-build-arm
   ```
2. **Plugin Tests**: Run tests for the newly migrated build plugin.
   ```bash
   ./dialtone.sh plugin test build
   ```
3. **Feature Tests**: Run tests for the cross-build-arm feature.
   ```bash
   ./dialtone.sh test cross-build-arm
   ```
4. **All Tests**: Run the entire test suite to ensure no regressions.
   ```bash
   ./dialtone.sh test
   ```

## Logging
1. Use the `src/logger.go` package to log messages.
2. Log the target architecture and the Podman command being executed.

## Subtask: Implementation
- description: [NEW/MODIFY] Migrate `build` logic from `src/build.go` to `src/plugins/build/cli/build.go` and delegate from `src/dev.go`.
- test: `./dialtone.sh build --help` works and is served from the plugin.
- status: done

## Subtask: Implementation
- description: [MODIFY] `src/plugins/install/cli/install.go`: Add support for installing cross-compilation tools and Podman if needed.
- test: `dialtone.sh install` can install `gcc-arm-linux-gnueabihf`, `gcc-aarch64-linux-gnu`, and `podman`.
- status: done

## Subtask: Research
- description: Review `src/plugins/build/cli/build.go` (after migration) and `buildWithPodman` implementation. Understand Podman flags for volume mounting in WSL.
- test: Documentation in Collaborative Notes about architecture-specific compiler names.
- status: done

## Subtask: Implementation
- description: [MODIFY] `src/plugins/build/cli/build.go`: Add flags for `--podman`, `--linux-arm`, and `--linux-arm64`.
- test: `RunBuild` correctly parses new flags.
- status: done

## Subtask: Implementation
- description: [MODIFY] `src/plugins/build/cli/build.go`: Refactor `buildWithPodman` to use target architecture and matching compiler.
- test: Podman command string contains correct `GOARCH` and `CC` for both arm and arm64 targets.
- status: done

## Subtask: Implementation
- description: [NEW] `tickets/cross-build-arm/test/integration_test.go`: Integration test to verify build flag logic and command construction.
- test: Test passes when running `./dialtone.sh test --ticket tickets/cross-build-arm`.
- status: done

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- test: All tests pass.
- status: done

## Collaborative Notes
- The user wants to build `src/dialtone.go` into a binary for Raspberry Pi with a camera.
- WSL integration: ensure `-v $(pwd):/src:Z` works correctly in WSL if Podman is installed.
- Compiler for 32-bit ARM: `arm-linux-gnueabihf-gcc`.
- Compiler for 64-bit ARM: `aarch64-linux-gnu-gcc`.
- Podman command should install the required compiler inside the `golang` container before running `go build`.
- Migration to plugin: Follow the pattern of `src/plugins/install/cli/install.go`.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
