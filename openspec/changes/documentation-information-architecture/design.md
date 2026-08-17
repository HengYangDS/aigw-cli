# Design

## Information Domains

| Domain | Owns |
| --- | --- |
| `architecture/` | Product boundaries, authority, security, and component relationships |
| `concepts/` | Product vocabulary and model strategy |
| `decisions/` | Durable decisions and their register |
| `evidence/` | Proof policy, release evidence, and bounded validation records |
| `experience/` | Human-facing terminal and text presentation contracts |
| `governance/` | Change, release, and adapter admission policy |
| `guides/` | Task-oriented adoption guidance |
| `operations/` | Forge and operator procedures |

## Navigation

`docs/README.md` is the only container index. It serves two paths:

1. task-oriented entry points for users and teams;
2. a compact reference map by information domain.

The decision register and evidence policy are content-bearing documents, not
directory placeholders. They retain their navigation where that navigation is
part of the document's own semantics. The Decision Record checker names the
register explicitly; it does not infer that every collection must use
`README.md`.

## Naming

Every document filename states its subject. A subdirectory does not gain a
`README.md` merely because it contains several files. A local index is justified
only when it owns additional semantics that cannot live in the global index or
an existing content-bearing register.

## Migration

Files move without compatibility copies. All current tracked links change in
the same commit, and the repository link gate proves closure.
