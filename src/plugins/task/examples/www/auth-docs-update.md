# auth-docs-update
### description:
Update Swagger/OpenAPI specs and internal docs for new Auth V2.
### tags:
- documentation
- api
### task-dependencies:
- auth-middleware-v2
### documentation:
- src/auth/docs/api.yaml
### test-condition-1:
`npm run lint:docs` passes.
### test-condition-2:
No broken links in generated site.
### test-command:
`npm run test:docs`
### reviewed:
- USER-1> 2026-02-06T13:30:00Z :: key-sig-8mn
### tested:
- LLM-TEST> 2026-02-06T13:35:00Z :: key-sig-9op
### last-error-types:
- BrokenLinkError
### last-error-times:
- BrokenLinkError: 2026-02-06T13:20:00Z
### log-stream-command:
`@DIALTONE npm run docs:serve`
### last-error-loglines:
- BrokenLinkError: "[WARN] Link to /auth/v1/login is broken"
### notes:
Publish to internal wiki after merge.
