## 1. Portability

- [x] 1.1 On a host without a declared client layout, make the host supply
      test verify the command's refusal and the interruption test skip; both
      pass on macOS ARM64 and, as a host without a layout, macOS AMD64.
- [x] 1.2 Keep the caller's Mise data and cache directories in the Mise
      configuration boundary test; it passes with tools installed only in a
      data directory outside the default data home.
- [x] 1.3 Make the selection preview test find the complete target path
      wherever the output wraps and name the check that failed; it passes at
      every temporary-directory length that failed before.
- [x] 1.4 Read release test files through the retrying reader and bound the
      PowerShell witness at sixty seconds; the release and projection tests
      pass.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head, and the full proof of the lane head passes.
- [ ] 2.2 The hosted graph passes at the lane head.
- [ ] 2.3 GitLab's protected `dev` and `main` pipelines pass for the published
      head.
