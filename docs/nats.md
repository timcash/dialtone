# NATS Messaging Architecture

NATS acts as the "central nervous system" of the robot. Telemetry (video, sensors) and commands (velocity, attitude) are published to a built-in NATS server.

## NATS Bridge

The system includes a Web Server & UI accessible via `http://<hostname>:80`. It provides:
- **NATS Bridge**: A WebSocket interface for interacting with the NATS bus directly from the browser.
- **Live MJPEG Stream**: Low-latency video feedback.
- **System Metrics**: Real-time stats on uptime, connection count, and throughput.
