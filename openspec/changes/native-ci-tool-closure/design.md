## Context

`.config/ci/pipeline.cue` already owns CI topology and projects both Forge
workflows. The native command reaches CUE through the Go CI driver, so the
required tools are a command closure rather than a language-only selection.

## Decision

Keep one small declarative value in the CI model:

```text
native acceptance -> go + cue
```

Generated Forge files consume that value unchanged. A focused test evaluates
the rendered projection, so drift in either Forge cannot become an independent
policy surface.

## Rejected Alternatives

| Alternative | Reason |
| --- | --- |
| Install every locked tool | Expands download, failure, and maintenance surface. |
| Duplicate per-Forge tool lists | Creates parallel authorities. |
| Skip CUE-backed checks in native jobs | Weakens the acceptance contract. |
