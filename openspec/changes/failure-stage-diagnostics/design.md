# Design

## Context

`internal/presentation` turns command errors into one problem. A
configuration `LoadError` already carries its phase (read, parse or validate),
and an `os.PathError` or `os.LinkError` carries its operation and
operating-system error, but the presentation drops both and prints a fixed
sentence. The credential command alone converts a `LoadError` into a
stage-specific problem with its program version. The presentation may import
only the configuration and secret packages, and the product spec forbids paths
in diagnostics.

## Goals / Non-Goals

**Goals:** every command names the failed stage, operation and
operating-system error, and the AIGW version for a configuration validation,
without a path or content; the credential reader and the CLI share one wording.

**Non-Goals:** changing which commands fail, adding retries, locating files by
path in output, or changing `aigw doctor`.

## Decisions

### One presentation owner for stage evidence

The presentation gains one helper that turns a configuration `LoadError`, an
`os.PathError` or an `os.LinkError` into a structured problem naming the stage
and the AIGW version that the caller passes. The CLI applies it with its build
version before rendering, and the credential reader builds its evidence with
the same wording. An error that already carries a configuration recovery
outcome or a structured problem keeps it.

### Path-free operating-system errors

The error text comes from a closed set: the error number's text, or "permission
denied", "no such file or directory" and "file exists" for the standard file
errors, and "unexpected error" otherwise. No caller-supplied text, path or
configuration content reaches the output.

## Risks / Trade-offs

- [An error number's text differs by platform] → the text names the
  operating-system error as that platform reports it, which is what an
  operator acts on.
- [A validation error hides its schema detail] → the stage and version locate
  the failure, and `aigw doctor` reports the detail locally.

## Migration Plan

Release with the next AIGW version; diagnostics change only their text. The
native pipelines on both peers confirm that aggregate coverage exceeds the
floor on Windows, which a local run cannot measure.

## Open Questions

- None.
