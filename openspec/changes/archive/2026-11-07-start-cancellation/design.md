## Context

See proposal.md, Why. `Runner.RunStream` in `internal/platform/process` is the
one owner of a captured child's lifecycle on every platform. It creates the
command with the caller's context, starts it through the platform start
(`startCapturedProcess`), waits, and joins `ctx.Err()` into a failed `Wait`.
The Windows start creates the child suspended, assigns it to its Job and
resumes it. `os/exec` kills the child when the context ends, at any point
after the child exists.

## Goals / Non-Goals

**Goals:**

- A deadline or cancellation that ends a captured child's start reaches the
  caller through `errors.Is`, whatever step of the start it interrupts.

**Non-Goals:**

- No change to the Windows start order, Job containment, resume rules or
  cleanup bounds.
- No new seam outside tests.

## Decisions

- **Join the context's error at the runner, not in the Windows start.**
  `RunStream` already reports cancellation after `Wait`. Joining `ctx.Err()`
  into a failed start gives both stages one rule in one place, and covers any
  start step that a terminated child fails, such as assignment ("Access is
  denied") or resume ("no thread to resume"). The alternative was to treat a
  child that ended before it resumed as started and let `Wait` report it. That
  needs another access right on the process handle, a new exit probe in the
  injected Windows API, and changes to the failure-boundary tests, for the
  same result at the caller.
- **Keep the start failure.** The joined error keeps the platform's cause next
  to the context's error, as the release-distribution requirement keeps
  failure causes. A start that fails while the context is live is unchanged.
- **Test through the native API value.** The Windows test replaces
  `nativeWindowsProcessAPI.resumeProcess` for its own run and restores it. The
  replacement cancels the context, waits for `os/exec` to terminate the
  suspended child, and then calls the native resume, which fails as it did in
  CI. The package's tests do not run in parallel.

## Risks / Trade-offs

- [A context that has already ended when `Start` runs makes `os/exec` return
  `ctx.Err()`, which the runner joins again] → The message repeats the
  cancellation; `errors.Is` is unchanged. `Wait` has the same property today.

## Migration Plan

None. The change ships with the next release, and reverting the commit
restores the previous report.
