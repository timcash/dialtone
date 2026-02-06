# audit-logger-service
### description:
Create a service to log critical security events to the database and external SIEM.
### tags:
- security
- logging
### task-dependencies:
- database-migration-users
### documentation:
- src/logging/audit.md
### test-condition-1:
Events persisted to `audit_logs` table.
### test-condition-2:
High severity events trigger alert hook.
### test-command:
`npm run test:audit`
### reviewed-at:
2026-02-06T14:05:00Z
### tested-at:
2026-02-06T14:30:00Z
### last-error-type:
DatabaseConstraintViolation
### last-error-time:
2026-02-06T14:20:00Z
### log-stream-command:
`@DIALTONE npm run audit:tail`
