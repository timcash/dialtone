# audit-logger-service
### description:
Create background service to log all V2 API accesses to data warehouse.
### tags:
- backend
- compliance
### task-dependencies:
- database-migration-users
### documentation:
- src/services/audit.js
### test-condition-1:
Logs appear in queue within 500ms.
### test-condition-2:
No PII is logged in plain text.
### test-command:
`npm run test:audit`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run service:audit --tail`
### last-error-loglines:
# None
### notes:
