# admin-stats-api
### description:
Create new admin endpoint for user growth stats using audit log data.
### tags:
- api
- admin
### task-dependencies:
- auth-middleware-v2
- audit-logger-service
### documentation:
- src/admin/stats.md
### test-condition-1:
GET /admin/stats/growth returns JSON.
### test-condition-2:
Requires Admin role.
### test-command:
`npm run test:admin`
### reviewed:
- USER-1> 2026-02-06T13:00:00Z :: key-sig-5gh
- LLM-CODE> 2026-02-06T13:10:00Z :: key-sig-6ij
### tested:
- LLM-TEST> 2026-02-06T13:15:00Z :: key-sig-7kl
### last-error-types:
- RoleMissing
### last-error-times:
- RoleMissing: 2026-02-06T12:55:00Z
### log-stream-command:
`@DIALTONE npm run server:admin --watch`
### last-error-loglines:
- RoleMissing: "AccessDenied: User does not have 'admin' role"
### notes:
Cache these stats for 1 hour.
