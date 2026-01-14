# System Design & Tech Stack

Dialtone is designed to run as a single-binary appliance on ARM64-based robotic platforms.

## Hardware Stack

- **The Robot**: Target platforms like Raspberry Pi 4/5 or NVIDIA Jetson. These handle the physical interaction with the environment.
- **Connected Devices**:
    - **Cameras**: Supports V4L2-compatible USB and MIPI cameras (e.g., Raspberry Pi Camera Module).
    - **Motors/Servos**: Interface via GPIO or serial bridges (e.g., MAVLink) integrated into the NATS bus.

## Software Stack

- **Control Computer**: A Go application that orchestrates the camera feed, NATS server, and web interface.
- **Web UI**: A real-time dashboard built with Vite/TypeScript and embedded directly into the Go binary.
