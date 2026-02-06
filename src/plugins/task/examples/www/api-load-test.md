# api-load-test
### description:
Run calibrated load tests against the V2 API endpoints.
### tags:
- testing
- performance
### task-dependencies:
- user-profile-api
- admin-stats-api
- rate-limiter-impl
### documentation:
- tests/load/k6-scripts.md
### test-condition-1:
P95 latency < 200ms at 1000 RPS.
### test-condition-2:
Error rate < 0.1%.
### test-command:
`npm run test:load`
### reviewed-at:
2026-02-06T14:40:00Z
### tested-at:
2026-02-06T14:45:00Z
### last-error-type:
ThresholdExceeded
### last-error-time:
2026-02-06T14:35:00Z
### log-stream-command:
`@DIALTONE npm run test:load --dashboard`
