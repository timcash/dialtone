#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "Updating Gemini CLI to the preview channel..."
npm install -g @google/gemini-cli@preview

echo "Starting Gemini CLI in YOLO mode with gemini-3.1-pro-preview..."
gemini --yolo --model gemini-3.1-pro-preview
