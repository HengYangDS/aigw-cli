## Decision

Construct one signed commit whose parent is current dev and whose product tree
matches the verified terminal tree. Retain only this commit on the Work Lane.

| Invariant | Evidence |
|---|---|
| One commit above dev | Git parent and revision count |
| Product behavior preserved | Tree comparison excluding the Change carrier |
| Trusted authorship | SSH commit signature |
| Shell automation removed | Tracked extension and shebang inventory |
| Independent Forges | CI and release contracts remain separate |
| Portable release staging | Releasekit materializes the runtime-only GoReleaser `dist` path in an isolated configuration |

No compatibility wrapper, forwarding layer, or alternate automation owner is
introduced.
