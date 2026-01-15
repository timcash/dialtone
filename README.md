# Dialtone

![Web Interface](ui.png)

Dialtone is a **robotic video operations network** designed to allow humans and AI to cooperatively train and control robots.

# Features
0. Simple single binary CLI to connect and control any robot
    - Cross platform support for Windows, MacOS, and Linux
    - Single command builds and deploys for ARM64 targets like Raspberry Pi
1. Built in virtual private network and peer discovery
    - Users on the network are identified by unique IDs
    - Access control lists for users and robots
2. Scalable command and control data structures
    - Request/reply for commands
    - Queuing for fanout and load balancing 
    - Streaming for live or replay of telemetry and video
4. Automated discovery and configuration
5. Vision and LLM AI assisted operation.
6. Language model tuned for development of the Dialtone system itself.

---

## 📚 Documentation Map

Detailed information about System Architecture, Installation, and Development can be found in the [docs/](./docs) directory:

- **[System Design & Tech Stack](./docs/techstack.md)**: Hardware/Software stack overview.
- **[Installation & Setup](./docs/install.md)**: Prerequisites and environment configuration.
- **[Build & Deployment](./docs/cli.md)**: Native and containerized builds, WSL support, and deployment commands.
- **[Development Workflow](./docs/develop.md)**: TDD loop, code style, and logging.
- **[Networking (Tailscale)](./docs/tsnet.md)**: Identity-based networking and automated provisioning.
- **[Messaging (NATS)](./docs/nats.md)**: System message bus and real-time telemetry.
- **[Testing Guide](./docs/test.md)**: Unit tests, integration tests, and UI screenshots.
- **[TODO](./todo.md)**: List of features to implement.

---

## 🚀 Quick Start (WSL/Linux No-Sudo)

The fastest way to get started on WSL or Linux without administrative privileges:

```bash
# 1. Install dependencies into ~/.dialtone_env (Go, Node, Zig, V4L2 headers)
go run . install-deps --linux-wsl

# 2. Perform a native full-build (includes Web UI and Camera support)
go run . full-build -local

# 3. Start the node locally
./bin/dialtone start -local-only
```

For standard ARM64 cross-compilation using Podman:

```bash
# Build the manager and deploy to a remote target
go build -o bin/dialtone .
bin/dialtone full-build
bin/dialtone deploy
```

## Why Dialtone uses Golang
1. Compiled language that produces single binary executables that are easy to deploy.
2. Support for concurrency.
3. Excellent standard library and strong ecosystem.
4. Good cross compilation support.
5. Typo safety and simple structure.
6. Strong networking support.
