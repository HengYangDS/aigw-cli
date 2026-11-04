## Context

Each native platform's GitLab job shares its CI VM with other projects'
jobs. On 2026-11-04 AIGW's Windows packages summed 1621–2604 seconds while
other jobs shared the VM, against 811–850 seconds on a quiet run. Three bounds
then failed correct heads: GitLab jobs 64866, 64985 and 64989. The retry 65081,
alone on the VM, passed the same head.

## Goals / Non-Goals

**Goals:**

- A test that is still making progress on a slow host passes.
- A process that hangs is still stopped, with Go's diagnostics, before the job
  itself runs out of time.

**Non-Goals:**

- Retrying failed tests in CI.
- Changing product timeouts, or the cleanup bounds that state how quickly the
  product reclaims its processes.
- Sizing or scheduling the CI hosts.

## Decisions

### A named hang bound for the selected fixture

The child run of `TestNativeProductSelectionDoesNotBuildUnselectedFixtures`
runs one selected fixture with `PATH` pointing at an empty directory. A fixture
that tries to build an unselected one therefore fails with an error, and the
time bound only has to stop a hang. Three minutes leaves room for a host many
times slower than the quiet run, whose fixture failed at 20 seconds at about
twice the quiet duration.

A share of the parent test's remaining deadline was rejected. The parent runs
six fixtures one after another, so a share would change with their order and
would still be one fixed value per run.

### Repeat an interruption that preceded its target

`TestWindowsExecutionPreflightReclaimsInterruptedProcesses` must interrupt a
native preflight after Hyperfine has started the workload. Otherwise it proves
nothing about reclaiming that workload. When an attempt ends before the
workload starts, the test repeats with a longer window, up to 9 seconds. The
workload sleeps for a minute, so every window still interrupts it.

Each attempt keeps the original cleanup bound of 2.7 seconds after its window,
and still requires every process it observed to have exited. A second
controller fails at once, because it would mean more than one Hyperfine run.

A product hook that signals the first descendant was rejected: it would add a
test seam to the product's observer for a timing condition of the
environment.

### The coverage policy declares the package limit

The coverage policy already owns how the coverage run executes, so it also
declares the per-package limit as `package_timeout`. The runner refuses a
missing, unparsable or non-positive value and passes it as `-timeout`.

Twenty minutes leaves room for three times the slowest quiet package,
`tools/release` at 190–224 seconds. It also stays below GitHub's 25-minute
native job budget, so a hang ends with Go's goroutine dump instead of a job
cancellation.

## Risks / Trade-offs

- A real hang now takes longer to surface: up to three minutes for one
  selected fixture and twenty for a package.
- On a slow host the interruption test can take up to about 21 seconds longer.
