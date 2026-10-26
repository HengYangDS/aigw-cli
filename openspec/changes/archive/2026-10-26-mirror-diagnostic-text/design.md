# Design

## Context

`assertMirrorFailureDiagnostics` counts lines that contain `mise WARN`. On
GitHub's hosted runners, Mise wraps each word of a diagnostic in ANSI escape
sequences, for example `ESC[33mmiseESC[0m ESC[33mWARNESC[0m`. The repository's
diagnostic classifier `process.DiagnosticFailure` already strips them with
`github.com/charmbracelet/x/ansi` before it reads a line.

## Goals / Non-Goals

**Goals:** the mirror test makes the same judgement on a colored hosted log and
an uncolored local terminal.

**Non-Goals:** changing which diagnostics count, or Mise's output settings.

## Decisions

### Strip presentation in the test, not in the environment

The test removes escape sequences with the same `ansi.Strip` before it counts
warnings.

Forcing `NO_COLOR` in the test's environment was rejected. A runner variable
that forces color, such as `CLICOLOR_FORCE`, can still win over it. Decoupling
the judgement from presentation holds whichever variable a runner sets. The
warning-counting logic becomes a small pure function, so the hosted output can
be tested directly.

## Risks / Trade-offs

- **A diagnostic that differs only in color is treated as the same text.**
  That is intended: color carries no meaning that the test asserts.

## Migration Plan

Publish the landed head to the 0.4.17 proposal so its checks rerun.

## Open Questions

- None.
