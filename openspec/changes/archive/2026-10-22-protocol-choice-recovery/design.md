# Design

## Context

`presentation.RenderError` renders a structured `userError` with its own
recommended action. Any other error gets a generic Problem whose action comes
from `suggestedFix`: the first quoted `aigw …` command in the message, or
`aigw check`. `resolveUseRuntime` in `internal/cli/selection` returns a plain
error for a non-interactive ambiguous protocol, and `planSetup` in
`internal/cli/onboarding` does the same for direct setup, so both fall back to
`aigw check`.

## Goals / Non-Goals

**Goals:** the recommended action of both refusals resolves the ambiguity, in
JSON and human output.

**Non-Goals:** changing which protocols are compatible, interactive selection,
the configuration-level resolver error, or other generic fallbacks.

## Decisions

### Return a structured Problem at the refusal

Both refusals return a Problem through the existing composition: `selection`
through `invocation.Problem`, `onboarding` through `presentation.ProblemError`,
as its other structured refusals do. The title keeps the current message, so
text that names `--protocol` is unchanged. Changing `suggestedFix` to infer
commands from prose was rejected: it would guess at every unclassified error.

### Name the exact retry where it is known

`aigw use` knows the client, Route and compatible protocols, so its action is
`aigw use --for <client> --protocol <a|b> <route>`. Direct setup does not keep
its full command line, so its action asks for the same setup command with
`--protocol <a|b>` rather than reconstructing flags.

## Risks / Trade-offs

- [The retry drops flags such as `--dry-run`] → the action names the selection
  itself; the refusal happened before any change, so the operator chooses the
  mode again.

## Migration Plan

None; the next release carries the new recommended actions.

## Open Questions

- None.
