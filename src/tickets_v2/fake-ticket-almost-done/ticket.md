# Name: fake-ticket-almost-done
# Tags: demo, almost-done

# Goal
Demonstrate a nearly complete ticket.

## SUBTASK: Backend API
- name: backend-api
- description: Implement the backend REST endpoints.
- test-condition-1: API responds with 200 OK
- agent-notes: 
- pass-timestamp: 2026-01-27T16:00:00-08:00
- status: done

## SUBTASK: Frontend UI
- name: frontend-ui
- dependencies: backend-api
- description: Create the user interface.
- test-condition-1: UI renders correctly
- agent-notes: 
- pass-timestamp: 2026-01-27T16:30:00-08:00
- status: done

## SUBTASK: Final Polish
- name: final-polish
- dependencies: frontend-ui
- description: Final UI/UX refinements.
- test-condition-1: UI is beautiful
- agent-notes: 
- status: todo
