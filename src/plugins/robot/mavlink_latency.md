# MAVLink Latency Report

This document maps the end-to-end journey of a telemetry message in the Dialtone Robot system, identifying every hop and the timestamp used to measure it.

## The 6-Step Journey

| Step | Location | Timestamp | Description |
| :--- | :--- | :--- | :--- |
| **1. Generation** | Flight Controller (FC) | `T_fc` | Message created by ArduPilot/PX4 firmware. Uses `time_boot_ms` (relative). |
| **2. Ingress** | Raspberry Pi (Go) | `t_raw` | `gomavlib` receives the raw frame from the serial port. First absolute sync point. |
| **3. Bus Entry** | Raspberry Pi (NATS) | `t_pub` | Message is parsed into JSON and published to the internal NATS bus. |
| **4. Relay** | Raspberry Pi (Web) | `t_relay` | WebSocket relay routine receives the NATS message and forwards it to the browser. |
| **5. Arrival** | Operator Browser | `now` | `ws.onmessage` event fires in the browser. |
| **6. Render** | 3D Dashboard UI | `t_ui` | React/Three.js updates the HUD legend and 3D model orientation. |

## Latency Metrics Calculation

The UI calculates the "P/Q/N" breakdown using these delta values:

### **P (Processing)**: `t_pub - t_raw`
*   **Location**: Internal to the Go process.
*   **Goal**: < 1ms.
*   **Measures**: Overhead of MAVLink parsing and JSON serialization.

### **Q (Queueing)**: `t_relay - t_pub`
*   **Location**: NATS Broker + Relay Loop.
*   **Goal**: < 5ms.
*   **Measures**: NATS delivery time and the responsiveness of the internal Go relay goroutine.

### **N (Network)**: `now - t_relay`
*   **Location**: The physical wire/air + Tailscale tunnel.
*   **Goal**: < 20ms (LAN), < 100ms (Tailscale).
*   **Measures**: Transit time from the Pi's web server to the browser. This is usually the largest component.

## Total End-to-End Latency
**Formula**: `now - t_raw`
This is the value displayed as the primary **LATENCY** in the HUD. It represents the total time from the message hitting the Raspberry Pi to it being available for rendering in your browser.

> **Note on Flight Controller Sync**: We currently do not subtract `T_fc` because flight controller clocks (`time_boot_ms`) are not synchronized with Unix time. The `t_raw` timestamp is used as the reliable "Ground Zero" for system-wide latency tracking.

## Diagnostic Tools

### 1. The 3D HUD Legend
Located in the bottom-right of the Three.js section.
*   **Sparkline**: Shows `Total` latency history.
*   **Values**: Shows `Total (P / Q / N)` in milliseconds.

### 2. CLI Telemetry Monitor
Run this on the robot to see P and Q in isolation:
```bash
./dialtone.sh robot telemetry
```

### 3. Log Inspection
Check the raw arrival times in the deployment log:
```bash
ssh tim@drone-1.dialtone.earth "tail -f ~/dialtone_deploy/robot.log | grep MAVLINK-RAW"
```
