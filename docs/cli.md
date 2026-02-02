# How to use `dialtone.sh` CLI and `git` for development
1. Use only these two tools as much as possible `dialtone.sh` CLI and `git`
2. Always run `./dialtone.sh ticket start <ticket-name>` before making any changes.
3. `dialtone.sh` is a simple wrapper around `src/dev.go`

## Installation & Setup
```bash
git pull origin main # update main so you can integrate it into your ticket
mv -n .env.example .env # Only if .env does not exists
./dialtone.sh install # Verify and install dev dependencies
./dialtone.sh install --remote # Verify and install dev dependencies on remote robot
```

## Ticket Lifecycle
```bash
./dialtone.sh ticket add <ticket-name> # Add a ticket.md to tickets/<ticket-name>/
./dialtone.sh ticket start <ticket-name> # Sets current ticket and scaffolds
./dialtone.sh ticket subtask list <ticket-name> # List all subtasks in tickets/<ticket-name>/ticket.md
./dialtone.sh ticket subtask next <ticket-name> # prints the next todo or process subtask for this ticket
./dialtone.sh ticket subtask test <ticket-name> <subtask-name> # Runs the subtask test
./dialtone.sh ticket subtask done <ticket-name> <subtask-name> # mark a subtask as done
./dialtone.sh ticket done <ticket-name>  # Final verification and completion
```

## Running Tests: Tests are the most important concept in `dialtone`
```bash
./dialtone.sh plugin test <plugin-name>                     # Run tests for a specific plugin
dialtone-dev test plugin <plugin-name> --list               # List tests that would run
dialtone-dev test tags <tag1> <tag2> ...                    # Run tests matching tags
dialtone-dev test ticket <ticket-name> [--subtask <name>]   # Run ticket or subtask tests
```

## Logs
```bash
./dialtone.sh logs # Tail and stream local logs
./dialtone.sh logs --remote # Tail and stream remote logs
./dialtone.sh logs --lines 10 # get the last 10 lines of local logs
./dialtone.sh logs --remote --lines 10 # get the last 10 lines of remote logs
```

## Plugin Lifecycle
```bash
./dialtone.sh plugin add <plugin-name> # Add a README.md to src/plugins/<plugin-name>/README.md
./dialtone.sh plugin install <plugin-name> # Install dependencies
./dialtone.sh plugin build <plugin-name> # Build the plugin
./dialtone.sh plugin test <plugin-name> # Runs tests in src/plugins/<plugin-name>/test/
```

## Build & Deploy
```bash
./dialtone.sh build --full  # Build Web UI + local CLI + robot binary
./dialtone.sh deploy        # Push to remote robot
./dialtone.sh diagnostic    # Run tests on remote robot (Requires ./dialtone.sh deploy first)
./dialtone.sh logs --remote # Stream remote logs
```

## GitHub & Pull Requests
```bash
./dialtone.sh github pr           # Create or update a pull request
./dialtone.sh github pr --draft   # Create as a draft
./dialtone.sh github check-deploy # Verify Vercel deployment status
```

## Git Workflow
```bash
git status                        # Check git status
git add .                         # Add all changes
git commit -m "feat|fix|chore|docs: description" # Commit changes
git push --set-upstream origin <branch-name> # push branch to remote first time
git push                          # Push updated branch to remote
git pull origin main              # Pull changes
git merge main                    # Merge main into current branch
```

## Develop the WWW site
```bash
./dialtone.sh www dev # Start local development server
./dialtone.sh www build # Build the project locally
./dialtone.sh www publish # Deploy the webpage to Vercel
./dialtone.sh www logs <deployment-url-or-id> # View deployment logs
./dialtone.sh www domain [deployment-url] # Manage the dialtone.earth domain alias
./dialtone.sh www login # Login to Vercel
```

## Develop the Web UI
```bash
./dialtone.sh ui dev          # Start local development server (vite)
./dialtone.sh ui build        # Build the production UI bundle
./dialtone.sh ui install      # Install frontend dependencies
./dialtone.sh ui mock-data    # Start a mock data server for testing telemetry/camera
./dialtone.sh plugin test ui  # Run integration tests for the UI
```

## AI Commands
```bash
./dialtone.sh ai opencode start   # Start AI assistant
./dialtone.sh ai developer        # Start autonomous developer loop
./dialtone.sh ai help             # Show all AI commands
```

## VPN & Provisioning
```bash
./dialtone.sh vpn provision --api-key <key> # Provision this device with Tailscale
./dialtone.sh vpn help                      # Show all VPN commands
```



