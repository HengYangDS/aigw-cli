## 1. Readiness model

- [x] 1.1 Extract one read-only active-route evaluation result from `check` and verify existing human output remains unchanged with readiness acceptance tests
- [x] 1.2 Add the `check --json` schema and renderer using the shared evaluation result; verify secret-free output and non-zero status for active-route failures

## 2. Verification

- [x] 2.1 Cover configured, deferred, missing-token, unavailable-adapter, and provider-failure journeys with acceptance tests
- [x] 2.2 Run `mise exec --locked -- go test ./internal/cli/... ./internal/configuration/...` and `mise exec --locked -- go run ./tools/ci source`
- [x] 2.3 Run the exact-head ETHOS proof for this change and record the receipt before landing
