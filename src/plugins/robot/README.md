# Robot Plugin

```bash
# QUICK START: All lifecycle commands (Must be validated)
./dialtone.sh robot install src_v1                 # Install UI dependencies
./dialtone.sh robot local-web-remote-robot src_v1  # Live UI with remote robot data
./dialtone.sh robot test src_v1                    # Run automated test suite
./dialtone.sh robot test src_v1 --attach           # Run tests and watch in browser
./dialtone.sh robot build src_v1                   # Build UI assets
./dialtone.sh robot build src_v1 --remote          # Sync and build on remote robot
./dialtone.sh robot serve src_v1                   # Serve built UI locally
./dialtone.sh robot sync-code src_v1               # Sync source code to robot
./dialtone.sh robot deploy src_v1                  # Build and ship to remote robot
./dialtone.sh robot deploy src_v1 --service        # Deploy as systemd service
./dialtone.sh robot deploy src_v1 --proxy          # Deploy with Cloudflare proxy
./dialtone.sh robot diagnostic src_v1              # Verify live robot UI/telemetry
```

The `robot` plugin is the central hub for all robot-specific logic, including MAVLink telemetry integration, NATS messaging, and the mobile-optimized 3D dashboard. 

The plugin is designed to be **self-contained**, minimizing dependencies on the Dialtone `core` by encapsulating build, installation, and deployment logic within versioned source directories (`src_v1`, etc.).

---

## 🛠 Development Lifecycle

Use these commands to develop new features, verify them locally, and ship them to the robot. All commands default to the latest `src_vN` version if not specified.

### 1. Environment Setup
Install all UI dependencies for the target version.
```bash
./dialtone.sh robot install src_v1
```

### 2. Live Development (Remote Robot)
Iterate on UI changes using real data from a remote robot without redeploying.
```bash
# Connect local UI to drone-1 (or set DIALTONE_HOSTNAME)
./dialtone.sh robot local-web-remote-robot src_v1
```

### 3. Verification & Testing
Run the full automated test suite to ensure telemetry, navigation, and controls are functional.
```bash
# Headless execution
./dialtone.sh robot test src_v1

# Watch playback in your dev browser
./dialtone.sh robot test src_v1 --attach
```

### 4. Build & Local Serving
Compile the UI assets and run the Go server locally.
```bash
./dialtone.sh robot build src_v1
./dialtone.sh robot serve src_v1
```

### 5. Remote Source Sync (Optional)
Sync the project source code to the robot to perform native builds on the hardware itself. This is useful for debugging CGO issues or if the cross-compilation environment is unavailable.
```bash
./dialtone.sh robot sync-code src_v1
```

### 6. Remote Build
Build the optimized binary directly on the remote robot. This command will first synchronize the necessary source code and then execute the build process on the robot via SSH.
```bash
./dialtone.sh robot build src_v1 --remote
```

### 7. Deployment
Build the optimized binary, auto-bump the UI version, and ship to the remote robot.
```bash
# Standard background deployment
./dialtone.sh robot deploy src_v1

# Deployment as a persistent systemd service
./dialtone.sh robot deploy src_v1 --service

# Deployment with Cloudflare Tunnel ( drone-1.dialtone.earth )
./dialtone.sh robot deploy src_v1 --proxy
```

### 6. Post-Deployment Diagnostic
Verify the live robot's UI and telemetry stream from your machine.
```bash
./dialtone.sh robot diagnostic src_v1
```

---

## 🏗 Modular Architecture

The plugin is split into specialized modules to ensure maintainability:

| File | Responsibility |
|------|----------------|
| `robot.go` | Entry point and subcommand router. |
| `start.go` | Core service logic (NATS, Web, MAVLink bridge). |
| `deploy.go` | SSH management, auto-versioning, and 4-step health checks. |
| `ops.go` | Version-aware router for `install`, `build`, `test`, etc. |
| `src_v1/cmd/ops/` | Version-specific implementation of CLI operations. |

---

## 🚀 Recent Improvements (Feb 2026)

- **6DOF Model Sync**: The 3D robot model now accurately reflects real-time MAVLink `ATTITUDE` (Roll, Pitch, Yaw).
- **Auto-Versioning**: Every deployment automatically increments the UI version (displayed in the header) for instant verification.
- **Optimized Footprint**: Binary size reduced to ~140MB by stripping symbols and excluding DuckDB (`-tags no_duckdb`).
- **Xterm Refinement**: Terminal now uses a full-screen **CSS Grid** layout with a sticky input bar, matching UIv2 standards.
- **MAVLink Bridge**: Bidirectional NATS bridge (`rover.command` -> MAVLink) for ARM, DISARM, and Mode switching.
- **Robust Health Checks**: Deployment now includes mandatory checks for Service Status, Internal Web (8080), NATS (4222), and Tailscale reachability.

---

## 🎯 The "Core-less" Vision

The `robot` plugin represents the future of the Dialtone CLI: a modular system where `src/core` contains only generic utilities, and all high-level functionality—from building binaries to managing hardware—lives within independent, versioned plugins. This allows us to scale to new robot generations (`src_v2`, `src_v3`) without bloating the main engine.
