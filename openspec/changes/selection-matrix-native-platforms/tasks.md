## 1. The matrix in the native jobs

- [x] 1.1 Give `ci native` a `--selection-matrix` flag, in its explicit and its GitLab environment form, that runs the
      quality graph's `selection-matrix` command after the Go source check. Done when native tests show the command
      once after the source check, once with `--full-quality`, and not for an artifact-only journey.
- [x] 1.2 Declare the quality job's platform once in the CUE graph, take both quality jobs' runners from it, and pass
      `--selection-matrix` in the native command of every other platform; regenerate the projections. Done when the
      projection check passes and a test of the generated pipelines finds the flag on exactly the native jobs whose
      runner differs from that Forge's quality job, on both Forges.
- [x] 1.3 Bound the matrix at 45 min and give every GitHub job that runs it 70 min and the proof's static gate 3,600 s.
      Done when a test of the tracked files finds every such job's and gate's budget above the bound.

## 2. Verification

- [ ] 2.1 Pass `mise run check` on the lane head.
- [x] 2.2 A fresh review of the lane's whole change by a reviewer who has not seen it. Done when its findings are
      answered in code or in this design.
- [x] 2.3 Pass the proposal's hosted checks on both Forges, with the matrix's tests in the macOS and Windows native
      jobs' output (`c980a8c49`: GitHub run 45455081134, GitLab pipeline 17879).
