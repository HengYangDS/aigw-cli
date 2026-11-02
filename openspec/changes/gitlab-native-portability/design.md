## Context

GitLab's native jobs run the same tests as GitHub's, on ARM64 hosts whose jobs
select their own Mise directories, with longer temporary paths and a Windows
runner that several jobs share. The failing tests assumed GitHub's hosts: a
declared client layout for the test host, Mise's default data directory, a
temporary path short enough to render on one line, and an idle Windows host.

## Goals / Non-Goals

**Goals:** the tests pass on every hosted platform and still verify what they
verified before.

**Non-Goals:** supplying native clients on Linux or Windows ARM64, changing
GitLab runner capacity or concurrency, and changing product behavior.

## Decisions

- **Assert the refusal rather than skip the host test.** The supply's refusal
  of a host without a declared layout is specified behavior, so on such a host
  the host test verifies it end to end through the command, including that no
  supply directory or export remains. The process-level interruption test
  cannot reach an installer there and skips; a unit test covers the refusal
  for each undeclared host, and an in-process test covers interruption, on
  every host.
- **Keep the caller's tool store and never install.** The probe was stopped
  while it installed Go into the default data directory that the test's
  environment filter had selected. The filter now keeps `MISE_DATA_DIR` and
  `MISE_CACHE_DIR` and still removes every other Mise setting, the test still
  directs Mise's state into its own workspace, and `MISE_AUTO_INSTALL=false`
  keeps a missing tool from being installed into the caller's store. The
  bound only stops a hang, so it is one minute, and a failure reports Mise's
  diagnostics. A longer bound alone would have let the test download Go and
  write outside the job's directories on persistent runners.
- **Compare without whitespace rather than render without a width.** The
  preview keeps the width that users see. The assertion removes the line
  breaks and indentation that wrapping inserts, as the command help tests do,
  and looks for the complete target path instead of its file name.
- **Use the existing retrying reader rather than a local loop.** On Windows
  `robustio` retries a sharing violation or a denied access for about two
  seconds, as when another process scans a new file; elsewhere it is a plain
  read. The readiness records of the helper processes go through it. The
  repository already uses it for removal and rename.
- **Bound the PowerShell witness at one minute.** The witness passed before
  the fifteen-second bound stopped it on the loaded Windows runner; the bound
  exists to stop a hang. A witness that outlives PowerShell keeps the output
  pipe open, so the wait for the pipe is bounded too.

## Risks / Trade-offs

- A hang in the PowerShell witness or in a Mise probe takes a minute to report.
- The process-level interruption test runs only on the Linux and macOS hosts
  whose native jobs supply clients.
- The GitLab hosts qualify the change only after publication, because GitLab
  runs protected pipelines for the published branches.
