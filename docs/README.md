# Documentation Root

Status: canonical.

Read one path, not the whole repository.

## Personal use

1. [Install AIGW](../README.md#install)
2. [Connect the first service](../README.md#first-connection)
3. [Use it every day](../README.md#every-day)
4. [Recover a local integration](governance/terminal-experience-contract.md#recovery-language)

## Team use

1. [Publish or import a token-free manifest](team-rollout.md)
2. [Understand secure local boundaries](security.md)
3. [Operate independent GitLab and GitHub release planes](operations/forge-operations.md)
4. [Review release evidence and its limits](release-readiness.md)

## Deeper references

| Surface | Owns |
| --- | --- |
| [Concepts](concepts.md) | Account, Profile, Route, Adapter, endpoint, and update model. |
| [Terminal experience contract](governance/terminal-experience-contract.md) | Task-first CLI navigation, narrow-terminal layout, and recovery language. |
| [Architecture boundary](architecture/authority-and-projection-boundary.md) | Control-plane boundaries and projection transaction model. |
| [Security model](security.md) | Credentials, local process boundaries, and real-request verification. |
| [Team rollout](team-rollout.md) | Team manifests, member setup, release artifacts, and updates. |
| [governance/](governance/change-and-release-policy.md) | Change, release, and contributor policy. |
| [decisions/](decisions/0001-control-plane-data-plane-boundary.md) | Durable design rulings. |
| [evidence/](evidence/README.md) | Verification records and limits. |
| [CHANGELOG](../CHANGELOG.md) | Published release history. |
| [LICENSE](../LICENSE) | MIT licensing terms for the repository. |

Code, tests, schemas, and CI outrank prose. AIGW-owned projections are
generated from canonical AIGW configuration; Codex runtime and proxy state are
evidence, not AIGW source of truth.
