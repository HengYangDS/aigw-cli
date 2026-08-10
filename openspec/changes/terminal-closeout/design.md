## Context

AIGW is already a portable Codex and Claude Code configuration control plane.
This Change owns the finite repository transformation that makes one local
release candidate ready. Landing, hosted CI, publication, installation, and
retirement consume that archived result; making them prerequisites of the
Change would create a lifecycle cycle.

## Decision

Use the existing semantic owners only. `go list -m -u` reports newer versions
for eight transitive modules, but `go mod why -m` proves the main module does
not need them and `go get -u all && go mod tidy` restores the current graph.
The correct latest-stable policy is therefore to keep every direct dependency
current and let direct owners select transitive versions; explicit transitive
pins would create a second dependency authority.

| Concern | Owner |
| --- | --- |
| Product upgrade and rollback | `internal/upgrade` |
| Repository release construction and publication | `tools/release` |
| Hosted/local source orchestration | `tools/ci` |
| Package and dependency topology | `tools/architecture` |
| Repository conformance | `tools/repository` |

Forge identity projection keeps one implementation and one remote transaction.
A `main` selection prepares both protected branches in isolated object storage,
validates both target histories, then publishes both refs with `git push
--atomic`; `proposal/*` remains the only single-branch form. No product-owned
rollback protocol or second publication stack is introduced.

Do not revive the rejected `selfupdate`, `releasekit`, shell-wrapper, or
candidate-carrier planes. Cross-package calls remain legal only in the declared
one-way dependency graph; composition and repository tools do not become
alternate domain owners.

## Verification

1. Audit direct stable Go updates and inspect every reported transitive delta.
2. Run the complete native source gate and architecture/conformance checks.
3. Build the complete local candidate and verify its reproducibility and
   installation contract.
4. Execute exact-HEAD ETHOS proof and archive the Change.
5. Land the archived result, then require native macOS, Linux, and Windows
   hosted evidence before either Forge release.
6. Publish and verify each Forge independently, install a released artifact,
   verify Codex and Claude Code projections, and retire absorbed lanes through
   public ETHOS commands.
