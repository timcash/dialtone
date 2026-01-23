---
trigger: always_on
---

How to use the `dialtone.sh` CLI for development

### 1. Installation & Setup
```bash
./dialtone.sh install             # Install tools
./dialtone.sh install --check     # Verify installation
```

### 2. Ticket Lifecycle
```bash
./dialtone.sh ticket start <name> # Start work (branch + scaffolding)
./dialtone.sh ticket done <name>  # Final verification before submission
```

### 3. Running Tests
Tests are your primary feedback loop.
```bash
./dialtone.sh ticket test <ticket-name> # Runs tests in tickets/<ticket-name>/test/
./dialtone.sh plugin test <plugin-name> # Runs tests in src/plugins/<plugin-name>/test/
./dialtone.sh test <tag1> <tag2> <tag3> # Scans all tests for any of these tags
./dialtone.sh test 
```

### 4. Build & Deploy
```bash
./dialtone.sh build --full  # Build Web UI + local CLI + robot binary
./dialtone.sh deploy        # Push to remote robot
./dialtone.sh diagnostic    # Run health checks on remote robot
./dialtone.sh logs --remote # Stream remote logs
```

### 5. GitHub & Pull Requests
```bash
./dialtone.sh github pr           # Create or update a pull request
./dialtone.sh github pr --draft   # Create as a draft
./dialtone.sh github check-deploy # Verify Vercel deployment status
```