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
- `npm run lint:docs` passes.
### test-condition-2:
- No broken links in generated site.
### test-command:
- `npm run test:docs`
### reviewed:
### tested:
### last-error-types:
### last-error-times:
### log-stream-command:
- `@DIALTONE npm run docs:serve`
### last-error-loglines:
### notes:
