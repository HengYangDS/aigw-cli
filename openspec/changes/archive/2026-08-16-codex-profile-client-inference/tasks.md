## 1. Specify the selection contract

- [x] 1.1 Define profile-driven client selection and fail-closed boundaries.
- [x] 1.2 Assign client-scope resolution to the configuration domain.

## 2. Implement with regression evidence

- [x] 2.1 Add failing configuration and command regressions.
- [x] 2.2 Implement the single configuration-owned selection operation.
- [x] 2.3 Pass focused configuration, test, and verify suites.

## 3. Verify and deliver

- [x] 3.1 Pass the complete local quality graph.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `profile-client-selection:An explicit client-scoped profile is self-describing` | `2.1` | `go test ./internal/configuration ./internal/cli/readiness ./internal/cli/acceptance` |
| `profile-client-selection:Ambiguous or conflicting selection fails closed` | `2.1` | `go test ./internal/configuration ./internal/cli/readiness ./internal/cli/acceptance` |
| `profile-client-selection:Unselected-profile behavior remains stable` | `2.3` | `go test ./internal/cli/readiness ./internal/cli/acceptance` |

## Delivery Boundary

Exact-HEAD proof, archive, land, hosted CI, publication, installation, and lane
retirement remain lifecycle effects owned by their public receipts.
