# Name: fake-ticket
# Tags: p0, demo, v2

# Goal
This is a fake ticket to demonstrate the ticket_v2 plugin functionality.
It covers various subtask states and dependencies.

## SUBTASK: Setup Env
- name: setup-env
- tags: setup
- description: Initialize the project environment.
- test-condition-1: config file exists
- agent-notes: 
- pass-timestamp: 2026-01-27T17:00:00-08:00
- fail-timestamp: 
- status: done

## SUBTASK: Build Core
- name: build-core
- tags: core
- dependencies: setup-env
- description: Build the main system binary.
- test-condition-1: binary is produced
- agent-notes: exit status 1
- pass-timestamp: 2026-01-27T17:12:53-08:00
- fail-timestamp: 2026-01-27T17:12:48-08:00
- status: done

## SUBTASK: Documentation
- name: documentation
- tags: docs
- dependencies: build-core
- description: Generate user documentation.
- test-condition-1: README is updated
- agent-notes: 
- pass-timestamp: 2026-01-27T17:12:59-08:00
- fail-timestamp: 
- status: done

