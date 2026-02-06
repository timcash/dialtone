# database-migration-users
### description:
Add `oauth_provider` and `profile_pic_url` columns to users table.
### tags:
- database
- migration
### task-dependencies:
# None
### documentation:
- src/db/migrations/005_users_v2.sql
### test-condition-1:
Migration script runs successfully.
### test-condition-2:
Rollback script reverts changes cleanly.
### test-command:
`npm run db:migrate:test`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run db:status`
### last-error-loglines:
# None
### notes:
