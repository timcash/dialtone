# Dialtone

![Web Interface](ui.png)

Dialtone is a **robotic video operator network** desined to allow humans and AI to cooperativly train and control robots.

# Features
0. Simple single binary CLI to connect and control any robot
1. Built in virtual private network
2. Scalable command and control with queueing, streaming, and telemetry
3. Real-time video streaming
4. Automated sensor and actuator discovery
5. Overlays opensource projects like ROS, mavlink, ardupilot, etc.

---

## 📚 Documentation Map

Detailed information about System Architecture, Installation, and Development can be found in the [docs/](./docs) directory:

- **[System Design & Tech Stack](./docs/techstack.md)**: Hardware/Software stack overview.
- **[Installation & Setup](./docs/install.md)**: Prerequisites and environment configuration.
- **[Build & Deployment](./docs/build.md)**: Containerized builds, ARM64 cross-compilation, and deployment commands.
- **[Development Workflow](./docs/develop.md)**: TDD loop, code style, and CLI options.
- **[Networking (Tailscale)](./docs/tsnet.md)**: Identity-based networking and automated provisioning.
- **[Messaging (NATS)](./docs/nats.md)**: System message bus and real-time telemetry.
- **[Testing Guide](./docs/test.md)**: Unit tests, integration tests, and UI screenshots.
- **[TODO](./todo.md)**: List of features to implement.
---

## 🚀 Quick Start

Build the manager and deploy to a remote target:

```bash
# 1a. Build the dialtone manager on windows
go build -o bin/dialtone.exe .

# 1b. Build the dialtone manager on linux/macos
go build -o bin/dialtone .

# 2. Perform a full build (Web + ARM64 binary)
bin/dialtone full-build

# 3. Deploy to the robot
bin/dialtone deploy

# 4. Tail remote logs
bin/dialtone logs
```
