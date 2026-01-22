# Branch: macos-install-build-deploy
# Task: Install, build, and deploy to the robot from macOS

> IMPORTANT: See `README.md` for the full ticket lifecycle and development workflow.
> Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Install dependencies on macOS (Apple Silicon) using `./dialtone.sh install`.
2. Perform a local build of the system using `./dialtone.sh build`.
3. Deploy the binary to a remote robot using `./dialtone.sh deploy`.
4. Verify the robot is running and logs are accessible.

## Non-Goals
1. DO NOT modify the install/build scripts unless bugs are found.
2. DO NOT implement new deployment features.

## Test
1. **Dependency Check**: Run `./dialtone.sh install --check` to verify all tools are present.
2. **Build Check**: Verify `bin/dialtone` exists after `./dialtone.sh build`.
3. **Deploy Check**: Run `./dialtone.sh logs --remote` after deployment to see the robot's heartbeat.

## Subtask: Installation
- description: [EXECUTE] Run `./dialtone.sh install --macos-arm` and update shell profile.
- test: `go version`, `node -v`, `zig version`, and `gh --version` work from the shell.
- status: done

## Subtask: Build
- description: [EXECUTE] Run `./dialtone.sh build`.
- test: `bin/dialtone --help` runs locally.
- status: done

## Subtask: Deployment
- description: [EXECUTE] Set `ROBOT_HOST`, `ROBOT_PASSWORD`, `DIALTONE_HOSTNAME`, and `TS_AUTHKEY`, then run `./dialtone.sh deploy`.
- test: `./dialtone.sh logs --remote` shows successful startup.
- status: done

## Collaborative Notes
- Ensure the robot is accessible via SSH and Tailscale (if using TS_AUTHKEY).
