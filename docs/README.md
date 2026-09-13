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

For verification, distinguish [source checks](../CONTRIBUTING.md#source-checks),
[native artifact lifecycle](../CONTRIBUTING.md#native-upgrade-acceptance),
[real-client execution](../CONTRIBUTING.md#real-client-acceptance), and
[historical upgrades](../CONTRIBUTING.md#historical-release-qualification).
They answer different questions; none substitutes for all the others.

## Reference map

- **Architecture:** [Authority and projection boundary](architecture/authority-and-projection-boundary.md)
  explains the control plane and projection transactions;
  [Security model](architecture/security-model.md) defines credentials, process
  boundaries and real-request verification.
- **Concepts:** [Product concepts](concepts/product-concepts.md) defines Accounts,
  Profiles, Routes, Adapters, endpoints and updates.
- **Decisions:** [Decision register](decisions/decision-register.md) indexes
  durable rulings and their rationale.
- **Experience:** [Terminal experience](experience/terminal-experience.md)
  defines navigation, layout and recovery language.
- **Governance:** [Text layout](governance/text-layout.md),
  [Adapter admission](governance/adapter-admission.md), and
  [Change and release policy](governance/change-and-release-policy.md) own
  contributor contracts.
- **Guides:** [Team rollout](guides/team-rollout.md) covers configuration
  distribution, member setup, release artifacts and updates.
- **Operations:** [Forge operations](operations/forge-operations.md) explains
  independent GitLab and GitHub publication.
- **Research:** [Provider tooling assessment](research/provider-tooling-assessment.md)
  compares needs, solution paradigms and practical implications.
- **History and legal:** [Changelog](../CHANGELOG.md) records releases;
  [License](../LICENSE) supplies the MIT terms.

Stable product and journey documentation lives here. Contributor evidence and
release acceptance live with the [change and release policy](governance/change-and-release-policy.md),
not in an independent evidence directory. Code, tests, schemas, and CI outrank
prose. AIGW-owned client artifacts are derived from canonical AIGW
configuration; client runtime and external service state are evidence, not
AIGW source of truth.
