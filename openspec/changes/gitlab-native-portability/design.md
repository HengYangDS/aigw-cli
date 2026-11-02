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
  supply directory or export remains. The interruption test cannot reach an
  installer there and skips; a unit test already covers the refusal for each
  undeclared host on every host.
- **Keep the caller's tool store rather than raise the bound.** The probe was
  stopped while it installed Go into the default data directory that the
  test's environment filter had selected. A longer bound would let the test
  download Go and write outside the job's directories on persistent runners.
  The filter now keeps `MISE_DATA_DIR` and `MISE_CACHE_DIR` and still removes
  every other Mise setting, and the test still directs Mise's state into its
  own workspace.
- **Compare without whitespace rather than render without a width.** The
  preview keeps the width that users see. The assertion removes the line
  breaks and indentation that wrapping inserts, as the command help tests do,
  and looks for the complete target path instead of its file name.
- **Use the existing retrying reader rather than a local loop.** `robustio`
  retries only transient Windows and macOS errors, such as a sharing violation
  while another process scans a new file, and is a plain read elsewhere. The
  repository already uses it for removal and rename.
- **Bound the PowerShell witness at sixty seconds.** The witness passed before
  the fifteen-second bound stopped it on the loaded Windows runner; the bound
  exists to stop a hang.

## Risks / Trade-offs

- A hang in the PowerShell witness takes sixty seconds to report.
- The interruption test does not run on hosts without a declared layout; it
  runs on every host whose native job supplies clients.
- The GitLab hosts qualify the change only after publication, because GitLab
  runs protected pipelines for the published branches.
