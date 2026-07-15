# Documentation Root

Status: canonical.

AIGW uses a small documentation kernel: stable architecture and policy,
durable decisions, dated evidence, and release history. Start with the product
concepts, then follow one narrow path rather than reading the whole tree.

| If you need | Start here |
| --- | --- |
| The product model and everyday concepts | [Concepts](concepts.md) |
| Secure setup and operating boundaries | [Security model](security.md) |
| Team rollout | [Team rollout](team-rollout.md) |
| Release or forge work | [Release evidence](release-readiness.md) and [Forge Operations](operations/forge-operations.md) |

| Surface | Owns |
| --- | --- |
| [architecture/](architecture/authority-and-projection-boundary.md) | Control-plane boundaries and projection transaction model. |
| [governance/](governance/change-and-release-policy.md) | Change, release, and contributor policy. |
| [Text Layout Policy](governance/text-layout-policy.md) | Portable whitespace and blank-line semantics. |
| [Forge Operations](operations/forge-operations.md) | Independent GitLab/GitHub forge operation and provenance. |
| [decisions/](decisions/0001-control-plane-data-plane-boundary.md) | Durable design rulings. |
| [evidence/](evidence/README.md) | Verification records and limits. |
| [CHANGELOG](../CHANGELOG.md) | Published release history. |
| [LICENSE](../LICENSE) | MIT licensing terms for the repository. |

Code, tests, schema, and CI outrank prose. AIGW-owned projections are generated
from canonical AIGW configuration; Codex runtime and proxy state are evidence,
not AIGW source of truth.
