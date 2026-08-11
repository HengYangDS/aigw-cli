# Design

## Principles

| Principle | Application |
| --- | --- |
| SSOT | Existing `.config`, ETHOS profile, Go tools, tests, and docs remain the only owners. |
| DRY | CI invokes one source gate; proof maps to the same owned commands. |
| MECE | Product, governance, publication, and host state keep separate authority. |
| Lean | Prefer deletion or extension of an existing owner over a new entity. |
| Portability | Repository-relative paths, Git text attributes, and native CI prove platform behavior. |

## Sequence

1. Close material, text-layout, commit, architecture, and proof contracts.
2. Refactor only where an executable gate demonstrates a real cohesion defect.
3. Run focused checks, the complete source gate, and exact-HEAD ETHOS proof.
4. Land once, publish independently, then retire absorbed residue.
