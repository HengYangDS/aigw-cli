## Context

`.config/ci/pipeline.cue` remains the single CI topology authority. Its generated
paths are repository coordinates, not host filesystem syntax.

The observed Windows failures had two causes:

| Boundary | Invalid assumption |
| --- | --- |
| Manifest path | `/` was compared with `filepath.Separator`. |
| CUE process | An absolute temporary model path was evaluated from a different volume. |

## Decision

Keep the model and projection list unchanged. Convert a validated
slash-separated repository path to host syntax only when joining it to the
repository root. Run CUE with the repository as `Cmd.Dir` and a relative model
argument.

## Rejected Alternatives

| Alternative | Reason |
| --- | --- |
| Replace `/` in every caller | Duplicates path policy and misses future callers. |
| Copy the CUE model into the tool directory | Creates another authority and temporary state. |
| Skip projection tests on Windows | Removes the native evidence that exposed the defect. |
