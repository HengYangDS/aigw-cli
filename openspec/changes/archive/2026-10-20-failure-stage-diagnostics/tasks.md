# Tasks

## 1. Native Command Coverage

- [x] 1.1 Cover the failure paths of the native job commands: the public input
      download and extraction, missing Git or Node.js, an incomplete client
      installation, defaulted trust, the Secret Service alias and GitHub
      environment writes; the CI-tool tests pass.

## 2. Diagnostics

- [x] 2.1 Make presentation and CLI tests expect the failed operation and
      error for local file failures and the failed stage for configuration
      loads; they fail before the change.
- [x] 2.2 Name the operation, error and stage in the diagnostics without paths
      or content; the tests pass.

## 3. Verification

- [x] 3.1 `mise run check` and the full proof of the lane head pass.
