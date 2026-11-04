## Why

On 2026-11-04 AIGW's GitLab Windows job failed three times on heads that
passed on every other host. Architecture Publisher's and DDWG's jobs ran on the
same four-vCPU CI Windows VM at the time, and AIGW's packages ran two to three
times slower than in a quiet run. Three fixed bounds then stopped tests that
were still making progress:

- the selected native fixture's 20-second child limit;
- an interruption after 300 milliseconds, which landed before Hyperfine had
  started the workload;
- Go's default 10-minute package limit in the coverage run.

Alone on the VM, the same head passed. A slow host thus turned correct heads
red and cost retries and other sessions' waiting time.

## What Changes

- Give the selected native fixture's child run a bound for hangs rather than
  for speed: three minutes, with its context fifteen seconds longer.
- Repeat the Windows interruption test with windows of 0.3, 1, 3 and 9 seconds
  until an interruption lands after the workload has started. Every attempt
  still proves that what it started is reclaimed within 2.7 seconds after its
  window.
- Declare a 20-minute package limit in the coverage policy, validate it, and
  pass it to `go test`, so no package depends on Go's default.
- Require hang bounds that tolerate a shared CI host.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: verification bounds leave room for a shared CI host
  instead of assuming a quiet one.

## Impact

Tests, the coverage policy and the coverage runner change. Product code, the
released programs, user configuration and credentials do not, so no release
follows.

The cleanup bounds that the Windows tests assert stay as they are, because
they describe how quickly the product reclaims its processes. A failing test
is not retried in CI. The CI hosts' capacity belongs to their operations, not
to this repository; the Windows VM moved to eight vCPUs on the same day.
