## Why

The exhaustive selection tests (the quality graph's `selection-matrix` gate, 4,582 leaf subtests of the selection
records written by every command) run in the full local proof and in each Forge's Linux quality job only. The native
macOS and Windows jobs of both Forges run the Go source check and the native coverage run, which leave the gate's
build tag out. So neither Forge has macOS or Windows evidence for these tests, although `AGENTS.md` requires native
evidence for the same product tree on macOS, Linux and Windows on each selected Forge.

The tests exercise file replacement, removal and directory creation, which differ by operating system. Measured on
2026-11-24 with the gate's own command, all passing: 573 s on hosted macOS and 837 s on hosted Windows (GitHub);
655 s and 855 s on the GitLab macOS and Windows guests and 118 s on its Linux guest (manual pipeline 17863); 138 s
on hosted Linux.

The owner decided on 2026-11-24 and 25: each Forge runs the same check classes on the same operating systems
(22:39; GitLab, on this ARM host, may lack x86); both Forges run the matrix on all three systems (23:57); and the
local proof keeps it, because the design holds with no remote, one or many (00:02 to 00:0x, 「1 本机照跑，远端都三系统」).

## What Changes

- Each Forge's native job on every platform other than the one its quality job runs on also runs the
  `selection-matrix` gate, once, from the quality graph's own declaration of the gate.
- The CUE graph declares the quality job's platform once; both Forges' quality jobs and the choice of native jobs that
  run the matrix derive from it.
- A native run that qualifies every quality tool (`--full-quality`) runs the gate once, through the quality graph; an
  artifact-only native journey runs no source tests and no matrix.
- The matrix's own bound falls from one hour to 45 minutes, three times its slowest measured run; every GitHub job that
  runs it gets 70 minutes and the local proof's static gate 3,600 s, so a hang ends with Go's diagnostics.
- The full local proof and the Linux quality jobs keep the gate unchanged.

No behavior of the product changes. Not breaking.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-quality`: the selection tests run in every full local proof and on every native platform that each Forge
  verifies, once per pipeline.

## Impact

`.config/ci/pipeline.cue` and its projections (`.gitlab-ci.yml`, `.github/workflows/verify.yml`), `tools/ci` (the
native command, the quality graph's matrix command and their tests, the projection tests), `.ethos/profile.toml`.
Each GitLab pipeline holds the macOS and Windows guests about 11 and 14 minutes longer (from the guest figures above);
GitHub's macOS and Windows native jobs grow by about 10 and 14 minutes.

Out of scope: the matrix's own size and the proof's 600 s goal (whether equivalent cases can merge is checked
separately, read-only first); ETHOS's gate scheduling; the proxy repository, whose GitLab peer has no macOS or Windows
jobs (ETHOS's R-25, review 2026-11-26).
