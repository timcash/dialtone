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
### reviewed:
- USER-1> 2026-02-06T14:10:00Z :: key-sig-cde
### tested:
- LLM-TEST> 2026-02-06T14:15:00Z :: key-sig-fgh
### last-error-types:
- ThresholdExceeded
### last-error-times:
- ThresholdExceeded: 2026-02-06T14:05:00Z
### log-stream-command:
`@DIALTONE npm run test:load --dashboard`
### last-error-loglines:
- ThresholdExceeded: "checks......: 95.00% ✓ 1342 ms  (expected < 200 ms)"
### notes:
Run during off-peak hours.
