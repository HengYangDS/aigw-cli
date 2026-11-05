## Context

`startCapturedWindowsProcess` in `internal/platform/process/capture_windows.go`
is the one owner of Windows child containment. It creates a Job Object with
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, starts the child, and assigns the child
to the Job. Closing the Job reclaims every descendant. The proposal names the
race between start and assignment.

## Goals / Non-Goals

**Goals:**

- The child is in its Job before it runs any code.
- Startup failures keep their cause and stop the child they started.

**Non-Goals:**

- Changing the Unix process-group path, the cleanup bounds, or the debug
  observer that `tools/release/performance` drives through `DEBUG_PROCESS`.

## Decisions

- **Suspended start.** The child is created with `CREATE_SUSPENDED`, added to
  the caller's own creation flags, then joins the Job, then resumes.
  Microsoft's `AssignProcessToJobObject` remarks describe a suspended creation
  for this purpose. A suspended child cannot exit before it is assigned, and
  it starts nothing outside the Job.
- **Resume through a thread snapshot.** `os/exec` closes the creation's thread
  handle, so `resumeSuspendedProcess` finds the child's thread with
  `CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD)` and resumes it through
  `OpenThread` and `ResumeThread`, all in `golang.org/x/sys/windows`. A
  suspended new process has exactly one thread.
- **Resume failure stops the child.** The existing startup cleanup stops a
  child whose resume fails, the same as one whose assignment fails, and the
  error keeps both causes.
- **Not an exited-child check.** Treating `ERROR_ACCESS_DENIED` from an exited
  child as success would hide the race, and it would leave the descendant gap
  open.

## Risks / Trade-offs

- A debuggee created with `DEBUG_PROCESS | CREATE_SUSPENDED` delivers its
  creation event only once it resumes. The observer attaches through `OnStart`
  after startup returns, so it still receives every event.
- These tests run only on Windows. The local graph cross-compiles and vets the
  Windows code; hosted Windows runs the tests.
