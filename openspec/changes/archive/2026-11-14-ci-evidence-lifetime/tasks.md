## 1. The lifetime

- [x] 1.1 Declare `evidenceRetentionDays` once in the CI model and carry it in every GitHub upload step and every
      GitLab artifact declaration; `go run ./tools/ci project --check` passes after regeneration.
- [x] 1.2 Add a projection test that fails for an upload step of any GitHub workflow or an artifact declaration
      without the declared lifetime; it fails when one step loses the field and when one declaration names
      another number.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [x] 2.2 The hosted graph passes at the proposal head, and one GitHub artifact and one GitLab job of it report
      the lifetime.
