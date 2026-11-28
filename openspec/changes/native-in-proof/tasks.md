## 1. The native test attempt

- [x] 1.1 Add `tools/ci native-tests` with optional JUnit and LCOV reports, running the test command that
      `nativeCommands` builds in native acceptance's isolated environment, and build `nativeCommands` from the same
      function; a test of the mode's command, environment, workspace cleanup and usage refusal passes.
- [x] 1.2 Make the quality graph's `coverage` gate and the proof's `go-behavior` gate run `tools/ci native-tests`,
      and remove the `native-resource-fixture` gate whose test that attempt runs; the tests that pin the source gate,
      the quality graph and the proof's behavior gate pass with the new commands.
- [x] 1.3 Describe in `CONTRIBUTING.md` that the source gate and the proof run the native test attempt; the prose gate
      passes.
