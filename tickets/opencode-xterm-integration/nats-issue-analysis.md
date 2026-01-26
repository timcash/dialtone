# NATS Connection Issue Analysis (UI Integration Tests)

## Current Status
While running `dialtone.sh test tags opencode`, the following symptoms are observed:
- **NATS Server**: Using a **REAL** embedded NATS server (`github.com/nats-io/nats-server/v2/server`) started in `mock.go`.
- **Symptoms**:
  - **Terminal Output**: `>>> NATS CONNECTION FAILED: Nats Error`
- **Console Log**: `WebSocket connection to 'ws://127.0.0.1:4223/' failed:`
- **Config Fetch**: `fetch('/api/init')` was failing with a `SyntaxError` (returning HTML instead of JSON).

## Suspected Issues

### 1. Missing `/api/init` in Mock Server
The web UI calls `fetch('/api/init')` to get configuration (version, ports, IPs). In the standalone `mock-data` server, this endpoint was not implemented, causing the UI to receive a 404 (rendered as HTML by Vite), leading to a JS crash before NATS could even connect.
- **Fix Status**: Implemented in `mock.go`.

### 2. Vite Proxy Configuration
The UI is served on `localhost:5174` (Vite), but the mock API and camera stream are on `localhost:8080`. Without a `vite.config.ts`, requests to `/api/*` were not being forwarded to the mock server.
- **Fix Status**: Created `src/core/web/vite.config.ts` with proxy rules.

### 3. Localhost (IPv4 vs IPv6) Ambiguity
Headless Chrome often struggles with `localhost` resolving to `::1` (IPv6) while the Go NATS server might only be listening on `127.0.0.1` (IPv4).
- **Fix Status**: Forced `127.0.0.1` in `main.ts` and `vite.config.ts`.

### 4. NATS WebSocket Handshake
- **Handshake Failure**: NATS server was logging `websocket handshake error: origin not allowed: not in the allowed list`.
- **Fix Status**: Explicitly set `AllowedOrigins: ["*"]` and `SameOrigin: false` in `mock.go`.
- **Current Status**: Still failing according to user. Possible causes:
  - Browser sending `null` origin or specific port that NATS `*` isn't catching (unlikely for NATS).
  - NATS requires explicit origins even with `*` in some configurations?
  - Port 4223 is listening but the upgrade is getting reset by Vite proxy? (Unlikely since UI connects directly to 4223).

## Next Steps
1. **Verify Proxy**: Ensure Vite is correctly forwarding `/api/init` and return valid JSON.
2. **Monitor NATS Logs**: Review the `Trace` and `Debug` logs from the NATS server to see if the WebSocket handshake is even reaching the server.
3. **JS Injection**: Use the injected JS in the test to log the *exact* response of the fetch and WebSocket attempt details.
