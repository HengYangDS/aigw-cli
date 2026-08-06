# Documentation Root

Status: canonical.

Read one path, not the whole repository.

## Personal use

1. [Install AIGW](../README.md#install)
2. [Connect the first service](../README.md#connect-a-service)
3. [Use it every day](../README.md#use-it-every-day)
4. [Recover a local integration](governance/terminal-experience-contract.md#boundary-language)

## Team use

1. [Publish or import a token-free manifest](guides/team-rollout.md)
2. [Understand secure local boundaries](governance/security.md)
3. [Operate independent GitLab and GitHub release planes](operations/forge-operations.md)
4. [Review release evidence and its limits](operations/release-readiness.md)

## Deeper references

| Surface | Owns |
| --- | --- |
| [Concepts](concepts/README.md) | Account, Profile, Route, Adapter, endpoint, and update model. |
| [Model strategy](concepts/model-strategy.md) | Curated capability set and admitted model baselines. |
| [Terminal experience contract](governance/terminal-experience-contract.md) | Task-first CLI navigation, narrow-terminal layout, and recovery language. |
| [Architecture boundary](architecture/authority-and-projection-boundary.md) | Control-plane boundaries and projection transaction model. |
| [Security model](governance/security.md) | Credentials, local process boundaries, and real-request verification. |
| [Adapter admission](governance/adapter-admission.md) | Evidence record for admitted adapters. |
| [Team rollout](guides/team-rollout.md) | Configuration manifests, member setup, release artifacts, and updates. |
| [governance/](governance/change-and-release-policy.md) | Change, release, and contributor policy. |
| [operations/](operations/forge-operations.md) | Forge operations and release evidence contracts. |
| [decisions/](decisions/0001-control-plane-data-plane-boundary.md) | Durable design rulings. |
| [evidence/](evidence/README.md) | Verification records, limits, and local Git-object housekeeping. |
| [CHANGELOG](../CHANGELOG.md) | Published release history. |
| [LICENSE](../LICENSE) | MIT licensing terms for the repository. |

Code, tests, schemas, and CI outrank prose. AIGW-owned client artifacts are
derived from canonical AIGW configuration; client runtime and external service
state are evidence, not AIGW source of truth.
