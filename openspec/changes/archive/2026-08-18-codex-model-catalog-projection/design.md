## Context

See `proposal.md`. Codex replaces, rather than merges, its bundled model table
when `model_catalog_json` is configured. A partial catalog would therefore fix
one alias while degrading every model omitted from it.

## Goals / Non-Goals

**Goals:**

- Preserve the complete bundled table and derive provider namespaces without a
  model-name registry.
- Keep generation, ownership, validation, rollback, and client evidence under
  existing semantic owners.
- Fail closed on foreign catalog files and stale client identity.

**Non-Goals:**

- Change wire model IDs, provider routing, credentials, Codex history, Desktop
  state, or the installed client's lifecycle.
- Silence arbitrary unknown-model warnings.

## Decisions

### Mirror the installed client's complete table

Clone every bundled entry and add aliases only when exactly one dot-separated
suffix matches a bundled slug. This preserves future fields and avoids a
hard-coded model list. A single-entry override was rejected because Codex uses
the file as a replacement table.

### Bind reuse to exact client identity

Record both client version and executable SHA-256. If regeneration fails, reuse
is allowed only for the same identity and unchanged owned bytes. Version-only
binding was rejected because different binaries can report the same version.

### Keep one compensated projection transaction

Write a new catalog before the configuration can reference it; remove a catalog
only after the reference and sidecar are withdrawn. The existing guarded
transaction owner performs all writes and rollback. A separate catalog service
or installer was rejected as an unnecessary authority surface.

### Measure client resolution without a model request

The developer command uses throwaway Codex homes and the client's read-only
debug surfaces to compare the base slug, unadapted prefix, adapted prefix, and
an unknown model. Fake-client tests cover deterministic behavior; real-client
evidence records the installed version and digest.

### Use aggregate coverage as the quantitative veto

Every package remains present, executed, and reported, while aggregate statement
and branch floors carry the merge decision. Reinstating a package-level veto
was rejected because small denominators recreate accidental complexity already
removed by the repository's rule-rationality design.

## Risks / Trade-offs

- **Client debug output changes** → The verification command fails explicitly
  and records the exact client identity rather than guessing.
- **A user occupies the managed catalog path** → Projection refuses to adopt,
  overwrite, re-permission, or delete the file.
- **Client upgrade makes a snapshot stale** → AIGW withdraws the reference and
  reports the stale state until regeneration succeeds.
