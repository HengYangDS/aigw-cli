# Tasks

- [x] 1.1 Replace concatenated package and tool names with the terminal semantic owners.
- [x] 1.2 Merge CI validation and execution into one command surface.
- [x] 1.3 Remove the release-tool to upgrade-runtime dependency.
- [x] 1.4 Add positive package-topology and dependency-direction architecture regressions.
- [x] 1.5 Update every local, CI, release, documentation, and build reference atomically.
- [ ] 1.6 Obtain exact hosted macOS, Linux, Windows, and Forge evidence.
- [ ] 1.7 Execute exact-HEAD proof and archive the Change.
- [ ] 1.8 Land the proven tree through the governed candidate train.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Enforced semantic ownership and quality` | `1.1` | `tools/architecture/behavior_test.go; go run ./tools/architecture --root .; go run ./tools/ci source` |
