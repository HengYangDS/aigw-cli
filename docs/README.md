# Documentation

Choose the shortest path for the task. This page is the only directory index;
each linked document owns one subject.

## Personal use

1. [Install AIGW](../README.md#install)
2. [Connect the first service](../README.md#connect-a-service)
3. [Use it every day](../README.md#use-it-every-day)
4. [Recover a local integration](experience/terminal-experience.md#boundary-language)

## Team use

1. [Publish or import a token-free manifest](guides/team-rollout.md)
2. [Understand secure local boundaries](architecture/security-model.md)
3. [Operate independent GitLab and GitHub release planes](operations/forge-operations.md)
4. [Review release evidence and its limits](governance/change-and-release-policy.md#quality-and-platform-evidence)

## Reference map

| Domain       | Document                                                                               | Owns                                                                   |
| ------------ | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Architecture | [Authority and projection boundary](architecture/authority-and-projection-boundary.md) | Control-plane boundaries and projection transaction model.             |
| Architecture | [Security model](architecture/security-model.md)                                       | Credentials, local process boundaries, and real-request verification.  |
| Concepts     | [Product concepts](concepts/product-concepts.md)                                       | Account, Profile, Route, Adapter, endpoint, and update model.          |
| Concepts     | [Model strategy](concepts/model-strategy.md)                                           | Curated capability set and admitted model baselines.                   |
| Decisions    | [Decision register](decisions/decision-register.md)                                    | Decision grammar, coverage rule, and durable rulings.                  |
| Experience   | [Terminal experience](experience/terminal-experience.md)                               | Task-first navigation, narrow-terminal layout, and recovery language.  |
| Experience   | [Text layout](experience/text-layout.md)                                               | Stable rendering and readable terminal presentation.                   |
| Governance   | [Adapter admission](governance/adapter-admission.md)                                   | Admission evidence for client adapters.                                |
| Governance   | [Change and release policy](governance/change-and-release-policy.md)                   | Change, release, proof, and closeout policy.                           |
| Guides       | [Team rollout](guides/team-rollout.md)                                                 | Configuration manifests, member setup, release artifacts, and updates. |
| Operations   | [Forge operations](operations/forge-operations.md)                                     | Independent GitLab and GitHub operation.                               |
| History      | [Changelog](../CHANGELOG.md)                                                           | Published release history.                                             |
| Legal        | [License](../LICENSE)                                                                  | MIT licensing terms.                                                   |

Code, tests, schemas, and CI outrank prose. AIGW-owned client artifacts are
derived from canonical AIGW configuration; client runtime and external service
state are evidence, not AIGW source of truth.
