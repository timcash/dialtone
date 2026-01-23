# Dialtone CLI Reference

```bash
# Clone the repo
git clone https://github.com/timcash/dialtone.git

# Fill in .env.example and rename it to .env 
# if .env does not exist
mv -n .env.example .env

# Install tools
./dialtone.sh install

# Verify installation
./dialtone.sh install --check

# Start work (branch + scaffolding)
./dialtone.sh ticket start <ticket-name>

# Final verification before submission
./dialtone.sh ticket done <ticket-name>

# Runs tests in tickets/<ticket-name>/test/
./dialtone.sh ticket test <ticket-name>

# Runs tests in src/plugins/<plugin-name>/test/
./dialtone.sh plugin test <plugin-name>

# Discovery across core, plugins, and tickets
./dialtone.sh test <feature-name>

# Run all tests
./dialtone.sh test

# Build Web UI + local CLI + robot binary
./dialtone.sh build --full

# Push to remote robot
./dialtone.sh deploy

# Run health checks
./dialtone.sh diagnostic

# Stream remote logs
./dialtone.sh logs --remote

# --- GitHub and git Commands ---

# Create or update a pull request
./dialtone.sh github pr

# Create as a draft
./dialtone.sh github pr --draft

# Verify Vercel deployment status
./dialtone.sh github check-deploy

# Git Hygiene
# Use git add to update git and ensure .gitignore is correct. Make atomic commits.
git add .
git commit -m "feat|fix|chore|docs: description"

# --- WWW Plugin Commands ---

# Deploy the webpage to Vercel
./dialtone.sh www publish

# Build the project locally
./dialtone.sh www build

# Start local development server
./dialtone.sh www dev

# View deployment logs
./dialtone.sh www logs <deployment-url-or-id>

# Manage the dialtone.earth domain alias
./dialtone.sh www domain [deployment-url]

# Login to Vercel
./dialtone.sh www login
```
