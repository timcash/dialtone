# VPN Plugin
The `vpn` plugin manages Tailscale-based encrypted overlays for Dialtone nodes. It has been migrated to the versioned source pattern (`src_vN`).

## Core Commands (Legacy & Relay)
These commands run the stable VPN relay and provisioning logic.

```bash
# Start the local VPN relay (exposes robot UI on http://127.0.0.1:8080)
./dialtone.sh vpn

# Generate a new Tailscale Auth Key
./dialtone.sh vpn provision

# Run a quick connectivity test
./dialtone.sh vpn test
```

## Versioned Source Commands (`src_v1`)
These commands manage the next-generation VPN dashboard and automated testing suite.

```bash
# Install UI dependencies
./dialtone.sh vpn install src_v1

# Run the automated browser test suite
./dialtone.sh vpn test src_v1

# Start the dashboard in development mode
./dialtone.sh vpn dev src_v1

# Build the production UI assets
./dialtone.sh vpn build src_v1
```

---

## Migration Status

### ✅ Working
- **Directory Structure**: `src_v1` is scaffolded with `cmd`, `ui`, and `test` directories mirroring the `template` plugin.
- **CLI Dispatcher**: `./dialtone.sh vpn` now correctly routes versioned commands to the `src_v1` logic.
- **UI Architecture**: The new dashboard uses `ui_v2` and features a 3D Mesh visualization and terminal.
- **Legacy Support**: The original VPN relay logic has been moved from the root core into the plugin while maintaining command compatibility.

### ❌ In Progress / Not Working
- **Import Cycle**: The `src_v1` Go code currently fails to compile because of a circular dependency between the root `dialtone` package and the plugin CLI. 
- **Shared Utilities**: Logic like `CreateWebHandler` and `CheckStaleHostname` is being moved to `src/core/util` to resolve the build error.

### 🛠 Left to Do
1. **Complete Refactoring**: Move the remaining shared web/network logic out of `src/dialtone.go`.
2. **Verify Build**: Ensure `./dialtone.sh vpn go-build src_v1` passes.
3. **Run Suite**: Execute the full `./dialtone.sh vpn test src_v1` and ensure all 12 steps pass.
4. **Final UI Polish**: Customize the 3D visualization specifically for VPN topology.
