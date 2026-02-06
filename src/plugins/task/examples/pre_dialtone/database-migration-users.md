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

### tested-at:

### last-error-type:

### last-error-time:

### log-stream-command:
`@DIALTONE npm run db:migrate --dry-run`
