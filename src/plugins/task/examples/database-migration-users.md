# database-migration-users
### description:
Create migration scripts to add OAuth provider columns to user table.
### tags:
- database
- migration
### task-dependencies:
[]
### documentation:
- src/db/schema.md
### test-condition-1:
Migration `20260206_add_oauth` executes successfully.
### test-condition-2:
Rollback works without data loss.
### test-command:
`npm run db:migrate:test`
### reviewed-at:
2026-02-06T13:15:00Z
### tested-at:
2026-02-06T13:20:00Z
### last-error-type:
MigrationTimeout
### last-error-time:
2026-02-06T13:18:00Z
### log-stream-command:
`@DIALTONE npm run db:migrate --dry-run`
