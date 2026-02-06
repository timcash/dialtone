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
- USER-1> 2026-02-06T12:00:00Z :: key-sig-567
- LLM-REVIEW> 2026-02-06T12:10:00Z :: key-sig-890
### tested:
- LLM-TEST> 2026-02-06T12:15:00Z :: key-sig-1ab
### last-error-types:
- QueueFullError
### last-error-times:
- QueueFullError: 2026-02-06T11:50:00Z
### log-stream-command:
`@DIALTONE npm run service:audit --tail`
### last-error-loglines:
- QueueFullError: "[SQS] Exceeded max batch size of 10"
### notes:
Log retention policy is 90 days.
