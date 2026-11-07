## Why

On Windows, AIGW starts a captured child suspended, assigns it to its Job
Object, and then resumes it. `os/exec` watches the child's context from the
moment the child exists. A deadline that ends inside that window terminates the
suspended child before it resumes. The start then fails with "suspended child
has no thread to resume", and `Runner.RunStream` returns that failure without
the deadline. After `Wait`, the same function already joins the context's
error.

The GitLab peer's main pipeline for `bacff084` hit this under host load.
`TestReleaseToolDeadlineLeavesItsParentLive` gives its child 100 ms and expects
`context.DeadlineExceeded`. The same code passed on 6 November. Native
credential operations are not affected, because they check their own context
first.

## What Changes

- `Runner.RunStream` joins the context's error into a failed start, as it does
  after `Wait`. A deadline or cancellation that ends the start then reaches the
  caller.
- A Windows test ends the context while the child is suspended and requires
  the cancellation.

Non-goals: the start order, Job containment and cleanup bounds do not change.
The same pipelines failed two deadline tests under host load: the setup
availability test and the macOS Keychain migration test. Both passed on the
same code at calmer times, so this Change does not touch them.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `release-distribution`: a deadline or cancellation that ends a tool's start
  reaches its caller.

## Impact

`internal/platform/process` changes on every platform, though only Windows has
the window. There is no CHANGELOG entry: the suspended start that opened the
window is unreleased, and the callers that users reach check their own context.
