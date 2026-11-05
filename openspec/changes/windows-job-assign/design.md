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
  the caller's own creation flags, then joins the Job, then resumes. A
  suspended child runs no code, so it cannot exit before it is assigned, and
  no process it starts itself lands outside the Job.
- **Resume through a thread snapshot.** `os/exec` closes the creation's thread
  handle, so `resumeSuspendedProcess` finds the child's threads with
  `CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD)` and resumes each through
  `OpenThread` and `ResumeThread` from `golang.org/x/sys/windows`. Because a
  thread identifier can be reused, it resumes a thread only after kernel32's
  `GetProcessIdOfThread` confirms the thread still belongs to the child.
- **Resume failure stops the child.** The existing startup cleanup stops a
  child whose resume fails, the same as one whose assignment fails, and the
  error keeps both causes.
- **Not an exited-child check.** Treating `ERROR_ACCESS_DENIED` from an exited
  child as success would hide the race, and it would leave the descendant gap
  open.

## Risks / Trade-offs

- A debuggee created with `DEBUG_PROCESS | CREATE_SUSPENDED` still raises its
  creation event before it runs any user-mode code, and holds it until the
  debugger continues. The observer, which waits through `OnStart` on the
  creating thread, therefore still receives every event.
- These tests run only on Windows. The local graph cross-compiles and vets the
  Windows code; hosted Windows runs the tests.
- Each Windows start now walks a snapshot of the system's threads. Credential
  reads run inside a 0.1 s budget, which the hosted performance journey
  measures.
- Processes that a child starts through another service, such as WMI, Task
  Scheduler or a COM server, are not its children and stay outside the Job.
