## Context

The architecture gate already owns the terminal text-layout rule. The only
nonconforming byte is one extra newline in the canonical specification.

## Goals / Non-Goals

**Goals:**

- Restore conformance with the existing owner gate.
- Preserve all specification semantics.

**Non-Goals:**

- Add a formatter, compatibility path, or second policy owner.
- Change product behavior or publication topology.

## Decisions

Remove only the extra newline. Reusing the existing architecture gate keeps one
policy owner and a smaller maintenance surface than adding another tool.

## Risks / Trade-offs

- **Accidental semantic edit** → review the one-line diff and run strict OpenSpec
  validation before exact proof.
