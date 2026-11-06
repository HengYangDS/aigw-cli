## Why

On Windows, AIGW starts every captured child, such as a client's credential
command, and only afterwards assigns it to the Job Object that owns its
cleanup. A child that exits at once makes the assignment fail with
"Access is denied". The start then reports a failure for a child that ran
correctly. Anything the child starts before the assignment also lands outside
the Job and survives cleanup.

GitHub's Windows runner hit this twice in two days.
`TestRetainedCredentialSurvivesInstalledExecutableUnlink` runs eight fast
`.cmd` credential commands at once, and it failed on PR 239 and on PR 241.
Neither change touched process code, and both runs passed when only the
failed job was rerun.

## What Changes

- A captured Windows child starts suspended, joins its Job, and only then
  resumes, so it runs no code outside the Job.
- A child that cannot resume is stopped, and startup reports why.
- Tests pin the order, the caller's creation flags, and the resume failure.

Non-goals: Unix process groups, the debug-observer contract, and cleanup
bounds do not change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `secret-storage`: a credential subprocess joins the process group or Job that
  owns its cleanup before it runs any code.

## Impact

`internal/platform/process` changes on Windows only, and the next release
ships the fix. Credential commands, clients and configuration do not change.
