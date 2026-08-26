## Context

AIGW generates a complete client-bound Codex model catalog so a
provider-prefixed model keeps the bundled metadata of its base slug. The former
verifier inferred resolution from prompt item counts and an exported private
configuration lockfile. Current Codex no longer exports that file.

## Goals / Non-Goals

**Goals:**

- Observe catalog loading through a current public Codex command.
- Compare complete model metadata without enumerating fields.
- Keep verification isolated, read-only, deterministic, and request-free.
- Report distinct facts for bundled presence, effective presence, and semantic
  identity.

**Non-Goals:**

- Verify provider reachability or send a billable request.
- Read or mutate the user's Codex Home, sessions, authentication, or Desktop
  state.
- Preserve the retired config-lockfile or prompt-shape heuristics.

## Decisions

### Observe the catalog directly

Use `codex debug models --bundled` for the native baseline and `codex debug
models -c model_catalog_json=<path>` under an empty temporary `CODEX_HOME` for
the effective projection. This is the narrowest public surface that directly
reports the object being verified.

### Compare complete metadata structurally

Remove only `slug` from the base and alias entries, encode the remaining JSON
canonically, and compare SHA-256 digests. This proves every current and future
metadata field without maintaining a parallel schema or relying on prompt
layout.

### Keep transport verification separate

Catalog loading and a real Responses request answer different questions. This
command proves the former without credentials or network traffic; the existing
client journey proves routing and transport independently.

## Risks / Trade-offs

- **The public catalog output changes shape** → decoding fails explicitly with
  the exact client identity instead of guessing.
- **The client ignores the configured catalog** → the alias is absent from the
  effective catalog and verification fails.
- **Alias metadata drifts** → the semantic digest differs and verification
  fails even when the alias remains present.
