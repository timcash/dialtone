# AGENT.md
## Learnings
### Authentication V2
- **Issue**: Token refresh logic had a race condition during concurrent requests.
- **Fix**: Implemented a mutex in `AuthService.refresh_token`.
- **Test Pattern**: Use `npm run test:auth:flaky` to catch these regressions.
- **Docs**: API spec for `/auth/refresh` updated in `src/auth/docs/api.yaml`.

## Common Commands
- **Test**: `npm run test:auth`
- **Lint**: `npm run lint:docs`
- **Deploy**: `npm run verify:staging`

## Active Context
- **Current Focus**: Monitoring V2 deployment stability.
- **Reference**: `src/plugins/www/task/auth-middleware-v2.md`
