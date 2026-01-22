# macOS Development Workflow

Follow these steps to install, build, and deploy the Dialtone project from a macOS (Apple Silicon) machine.

```bash
# 1. Install local tools (Go, Node.js, Zig, etc.) for macOS ARM
./dialtone.sh install --macos-arm

# 2. Verify all required tools are correctly installed
./dialtone.sh install --check

# 3. Perform a full build including Web UI assets
# (Use --full to ensure web assets are rebuilt when changed)
./dialtone.sh build --full

# 4. Deploy the compiled binary and assets to the robot
./dialtone.sh deploy

# 5. Run health diagnostics (local & remote)
./dialtone.sh diagnostic

# 6. Stream remote logs to verify operational status
./dialtone.sh logs --remote
```
