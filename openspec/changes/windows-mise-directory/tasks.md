## 1. Directory

- [x] 1.1 Install the pinned Mise 2026.10.25 executable and shim in
      `C:\Program Files\mise-aigw-cli\bin` on the GitLab Windows ARM64 host in
      a window without running jobs, both digests verified against the CUE
      pins (2026-11-03 20:08; the shared directory is unchanged).
- [x] 1.2 Point the GitLab Windows Mise preparation at that directory through
      one CUE value; the projection test and the retained-step record follow,
      and `tools/ci project --check` reports no drift.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (`1ee8191d`).
- [x] 2.2 A GitLab review pipeline of the lane head runs `native-windows-review`
      with the new directory and passes (`1ee8191d`, MR !182, pipeline 13962,
      job 64337).
