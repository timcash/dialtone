#!/bin/bash
set -e

echo "Starting verification for 'ide setup-workflows'..."

# Path to the .agent directories
WORKFLOWS_DIR=".agent/workflows"
RULES_DIR=".agent/rules"

# 1. Clean up existing .agent files for a fresh test
echo "Cleanup: Removing existing .agent/workflows and .agent/rules..."
rm -rf "$WORKFLOWS_DIR"
rm -rf "$RULES_DIR"

# 2. Run the command
echo "Step 1: Running './dialtone.sh ide setup-workflows'..."
./dialtone.sh ide setup-workflows

# 3. Verify symlinks exist
echo "Step 2: Verifying symlinks..."
if [ -L "$WORKFLOWS_DIR/ticket_workflow.md" ]; then
    echo "SUCCESS: $WORKFLOWS_DIR/ticket_workflow.md is a symlink."
else
    echo "FAILURE: $WORKFLOWS_DIR/ticket_workflow.md is NOT a symlink or does not exist."
    ls -la "$WORKFLOWS_DIR"
    exit 1
fi

if [ -L "$RULES_DIR/rule-code-style.md" ]; then
    echo "SUCCESS: $RULES_DIR/rule-code-style.md is a symlink."
else
    echo "FAILURE: $RULES_DIR/rule-code-style.md is NOT a symlink or does not exist."
    ls -la "$RULES_DIR"
    exit 1
fi

# 4. Verify it fails if files exist
echo "Step 3: Running again to verify failure on existing files..."
if ./dialtone.sh ide setup-workflows 2>&1 | grep "File already exists"; then
    echo "SUCCESS: Command failed as expected when files exist."
else
    echo "FAILURE: Command did not fail or error message mismatch."
    exit 1
fi

echo "Verification complete! All tests passed."
