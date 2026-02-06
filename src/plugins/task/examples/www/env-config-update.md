# env-config-update
### description:
Update environment variables for the new V2 Auth Service.
### tags:
- config
- devops
### task-dependencies:
# None
### documentation:
- src/config/README.md
### test-condition-1:
`process.env.AUTH_V2_ENABLED` is true.
### test-condition-2:
Secret keys are loaded from Vault.
### test-command:
`npm run config:validate`
### reviewed:
- USER-1> 2026-02-06T10:00:00Z :: key-sig-123
- LLM-CODE> 2026-02-06T10:05:00Z :: key-sig-456
### tested:
- LLM-TEST> 2026-02-06T10:10:00Z :: key-sig-789
### last-error-types:
- ConfigValidationError
### last-error-times:
- ConfigValidationError: 2026-02-06T09:55:00Z
### log-stream-command:
`@DIALTONE npm run config:print`
### last-error-loglines:
- ConfigValidationError: "[FATAL] Missing required key: AUTH_JWT_SECRET"
### notes:
This must be deployed to all regions before services restart.
