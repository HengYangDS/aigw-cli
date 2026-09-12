# Documentation

Choose the audience and follow one journey. This page is the only documentation
directory index; each linked document owns one subject.

Place a document by its responsibility, not the task that produced it:
`research` records observations and comparisons; `decisions` records adopted
choices and their rationale; `architecture` explains the current design.
`concepts` defines the product vocabulary, `experience` specifies interaction,
`guides` teach user journeys, `operations` covers publication procedures, and
`governance` owns contributor policy. Active intent and progress belong to
OpenSpec; generated verification output does not become current documentation.

## Operators

| Journey                                             | Start                                                                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Evaluate product scope and ownership                | [Project overview](../README.md) and [authority boundary](architecture/authority-and-projection-boundary.md) |
| Compare existing configuration and gateway products | [Competitor capabilities and evidence](research/provider-tooling-assessment.md)                              |
| Install or uninstall                                | [Portable installation lifecycle](../README.md#install)                                                      |
| Connect the first service                           | [Interactive or manifest setup](../README.md#connect-a-service)                                              |
| Select and verify daily use                         | [Daily commands](../README.md#use-it-every-day)                                                              |
| Add a client after setup                            | [Deferred client synchronization](guides/team-rollout.md#install-a-client-later)                             |
| Choose a credential backend                         | [Credential storage](architecture/security-model.md#credential-storage)                                      |
| Diagnose and recover                                | [Troubleshooting journey](experience/terminal-experience.md#troubleshooting-journey)                         |
| Use a direct or compatibility URL                   | [External endpoint boundary](architecture/authority-and-projection-boundary.md#external-endpoints)           |

## Team maintainers

| Journey                          | Start                                                                                                  |
| -------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Review and distribute capability | [Team rollout](guides/team-rollout.md)                                                                 |
| Understand security boundaries   | [Security model](architecture/security-model.md)                                                       |
| Operate GitLab and GitHub        | [Forge operations](operations/forge-operations.md)                                                     |
| Review release evidence          | [Quality and platform evidence](governance/change-and-release-policy.md#quality-and-platform-evidence) |

## Contributors and extenders

| Journey                         | Start                                                                                |
| ------------------------------- | ------------------------------------------------------------------------------------ |
| Prepare a development Work Lane | [Contribution workflow](../CONTRIBUTING.md)                                          |
| Add a Provider or model         | [Extension model](architecture/authority-and-projection-boundary.md#extension-model) |
| Add a client                    | [Adapter admission](governance/adapter-admission.md)                                 |
| Understand durable decisions    | [Decision register](decisions/decision-register.md)                                  |
| Change or release the product   | [Change and release policy](governance/change-and-release-policy.md)                 |

## Reference map

| Domain       | Document                                                                               | Owns                                                                   |
| ------------ | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Architecture | [Authority and projection boundary](architecture/authority-and-projection-boundary.md) | Control-plane boundaries and projection transaction model.             |
| Architecture | [Security model](architecture/security-model.md)                                       | Credentials, local process boundaries, and real-request verification.  |
| Concepts     | [Product concepts](concepts/product-concepts.md)                                       | Account, Profile, Route, Adapter, endpoint, and update model.          |
| Decisions    | [Decision register](decisions/decision-register.md)                                    | Decision grammar, coverage rule, and durable rulings.                  |
| Experience   | [Terminal experience](experience/terminal-experience.md)                               | Task-first navigation, narrow-terminal layout, and recovery language.  |
| Governance   | [Text layout](governance/text-layout.md)                                               | Repository-wide text, documentation and formatter ownership.           |
| Governance   | [Adapter admission](governance/adapter-admission.md)                                   | Admission evidence for client adapters.                                |
| Governance   | [Change and release policy](governance/change-and-release-policy.md)                   | Change, release, proof, and closeout policy.                           |
| Guides       | [Team rollout](guides/team-rollout.md)                                                 | Configuration manifests, member setup, release artifacts, and updates. |
| Operations   | [Forge operations](operations/forge-operations.md)                                     | Independent GitLab and GitHub operation.                               |
| Research     | [Provider tooling assessment](research/provider-tooling-assessment.md)                 | Needs, solution paradigms, competitive value, and implications.        |
| History      | [Changelog](../CHANGELOG.md)                                                           | Published release history.                                             |
| Legal        | [License](../LICENSE)                                                                  | MIT licensing terms.                                                   |

Stable product and journey documentation lives here. Contributor evidence and
release acceptance live with the [change and release policy](governance/change-and-release-policy.md),
not in an independent evidence directory. Code, tests, schemas, and CI outrank
prose. AIGW-owned client artifacts are derived from canonical AIGW
configuration; client runtime and external service state are evidence, not
AIGW source of truth.
