# Documentation Root

Status: canonical.

AIGW uses a small documentation kernel: stable architecture and policy,
durable decisions, dated evidence, and release history. It borrows this
structure to keep ownership and proof legible; it does not import another
project's product model.

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
