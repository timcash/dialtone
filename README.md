# Dialtone

![Web Interface](ui.png)

Dialtone is a **robotic video operations network** desined to allow humans and AI to cooperativly train and control robots.

# Features
0. Simple single binary CLI to connect and control any robot
    - Cross platform support for Windows, MacOS, and Linux
    - Single command builds and deploys for ARM64 targets like Raspberry Pi
1. Built in virtual private network and peer discovery
    - Users on the network are identified by unique IDs
    - Access control lists for users and robots
2. Scalable command and control datastructers
    - Request/reply for commands
    - Queueing for fanout and load balancing 
    - Streaming for live or replay of telemetry and video
4. Automated discovery and configuration
    - Sensors like cameras, lidars, gps, imu  
    - Actuators like mortors and servos
    - Compute resources like flight controllers, gpus and FPGA's
    - Power resources like batteries and solar panels
5. Overlays opensource projects
    - Use ROS drivers for indrustrial robots like Fanuc, 
    - mavlink, ardupilot, etc.
6. Visson and LLM AI assisted operation focused on configuration, telemetry, anomaly detection, and autonomous operation.
   - Object detection and tracking
   - Path planning and navigation
   - Autonomous takeoff and landing
   - Obstacle avoidance
   - Visual SLAM
7. Language model tuned for developemnt of the dialtone system itself.

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

## Why Dialtone uses Golang
1. Compiled language that produces single binary executables that are easy to deploy.
2. Support for concurrency which is important for robotic systems that need to handle
3. Excelelent standard library and a strong ecosystem of third party libraries.
4. Golang has good cross compilation support which is important for building for ARM64 systems like Raspberry Pi.
5. Large projects with many contributors are possible with simple code structure and strong typing.
6. Networking support is a crtical aspect of dialtone
