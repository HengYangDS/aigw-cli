## Context

AIGW is already a portable Codex and Claude Code configuration control plane.
The remaining work is closure: remove placeholder truth, refresh the stable
locked graph, prove all supported platforms, publish two independent histories,
and retire absorbed lanes through the adopted ETHOS command.

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
3. Execute exact-HEAD ETHOS proof and archive the Change.
4. Land, then require native macOS, Linux, and Windows hosted evidence.
5. Publish and verify GitLab and GitHub independently.
6. Install the accepted artifact, verify Codex and Claude Code projections, and
   retire every absorbed lane using public ETHOS commands.
