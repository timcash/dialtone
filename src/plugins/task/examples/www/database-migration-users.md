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
- Migration script runs successfully.
### test-condition-2:
- Rollback script reverts changes cleanly.
### test-command:
- `npm run db:migrate:test`
### reviewed:
- USER-1> 2026-02-06T10:15:00Z :: key-sig-abc
- LLM-CODE> 2026-02-06T10:20:00Z :: key-sig-def
### tested:
- LLM-TEST> 2026-02-06T10:25:00Z :: key-sig-ghi
### last-error-types:
- SyntaxError
### last-error-times:
- SyntaxError: 2026-02-06T10:12:00Z
### log-stream-command:
- `@DIALTONE npm run db:status`
### last-error-loglines:
- SyntaxError: "PG::SyntaxError: ERROR:  syntax error at or near 'VARCHAR'"
### notes:
Backup the user table before running in production.
