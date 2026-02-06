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

### tested-at:

### last-error-type:

### last-error-time:

### log-stream-command:
`@DIALTONE npm run server:admin --watch`
