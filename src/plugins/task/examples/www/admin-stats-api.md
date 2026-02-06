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
### reviewed-at:
2026-02-06T14:15:00Z
### tested-at:
2026-02-06T14:40:00Z
### last-error-type:
RoleMissing
### last-error-time:
2026-02-06T14:30:00Z
### log-stream-command:
`@DIALTONE npm run server:admin --watch`
