## 1. Release transition

- [x] 1.1 Add a failing governance regression for the missing transition.
- [x] 1.2 Declare the proof-bound accepted-to-release edge.
- [x] 1.3 Enforce the declaration in repository governance.
- [x] 1.4 Pass focused and repository gates.
- [x] 1.5 Prove, land, close out, and promote accepted source to main.

## Delivery Boundary

Hosted CI, independent Forge publication, release assets, installation,
runtime acceptance, and lane retirement remain separately proven delivery
effects after this Change lands.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `repository-organization:Governed release-branch convergence` | `1.1` | focused red/green `TestGovernanceRequiresProofBoundReleaseTransition` |
| `repository-organization:Governed release-branch convergence` | `1.2` | tracked `.ethos/workspace.toml` edge |
| `repository-organization:Governed release-branch convergence` | `1.3` | `checkReleaseTransition` repository gate |
| `repository-organization:Governed release-branch convergence` | `1.4` | OpenSpec strict, repository tests, and static source gate |
| `repository-organization:Governed release-branch convergence` | `1.5` | exact-HEAD proof and public transition receipts |
