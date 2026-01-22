# Branch: macos-install-build-deploy
# Task: Install, build, and deploy to the robot from macOS

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Install dependencies on macOS (Apple Silicon) using `./dialtone.sh install`.
2. Perform a local build of the system using `./dialtone.sh build`
2. Make a build locally using zig on macos without podman using `./dialtone.sh build --arm64`. the `--arm64` flag may be incorrect. I want to build for the raspberry pi on the robot but from the mac.
3. Deploy the binary to a remote robot using `./dialtone.sh deploy`.
4. Improve the diagnostics output of `./dialtone.sh diagnostics` and look for errors
5. Get logs via `./dialtone.sh logs --remote`.

## Test
1. **Dependency Check**: Run `./dialtone.sh install --check` to verify all tools are present.
2. **Build Check**: Verify `bin/dialtone` exists after `./dialtone.sh build`.
3. **Deploy Check**: Run `./dialtone.sh logs --remote` after deployment to see the robot's heartbeat.

## Subtask: Installation
- description: [EXECUTE] Run `./dialtone.sh install --macos-arm` and update shell profile.
- test: test that `./dialtone.sh` loads env vars correctly.
- status: done

## Subtask: Build
- description: [EXECUTE] Run `./dialtone.sh build --arm64`.
- test: `bin/dialtone --help` runs locally.
- status: done

## Subtask: Deployment
- description: [EXECUTE] Run `./dialtone.sh deploy`.
- test: `./dialtone.sh logs --remote` shows successful startup.
- status: done
