## 1. Specifications

- [x] 1.1 Write the deltas of the twelve capabilities that the proposal lists. Done when
      `openspec validate drop-windows-and-intel-macos --strict` passes and the lane's archive rehearsal reports every
      merged specification within the size limit.

## 2. Release matrix

- [x] 2.1 Reduce `.config/release/goreleaser/.goreleaser.yaml` and `internal/program/artifact/identity.go` to
      `darwin-arm64`, `linux-arm64` and `linux-amd64`, with a cask whose only macOS archive is ARM64 and no zip or `.exe` path, and make
      the tests that enumerate targets follow (D1). Done when GoReleaser's `check` accepts the configuration and the
      package tests of `internal/program/...` and `tools/release/...` that read the matrix pass.

## 3. Product code

- [x] 3.1 Delete the Windows token backends (Credential Manager, DPAPI file) and the direct
      `github.com/danieljoos/wincred` requirement (D4). Done when `go mod tidy -diff` prints nothing.
- [x] 3.2 Delete every Windows file, build constraint, branch, test case and fixture under `internal/` and `cmd/`; a
      POSIX twin takes the plain name (D4). Done when `go build` and `go vet` pass for darwin and linux on arm64 and
      amd64 and no Go file under those roots names Windows.
- [x] 3.3 Collapse each seam whose only second implementation was Windows after confirming it in the code: the
      `internal/secret/native` dispatch, `terminal.ColorEnabled`'s operating-system parameters and
      `process.Plan.DebugProcess` (D4). Done when each has one implementation and the package tests of
      `internal/secret/...`, `internal/terminal` and `internal/platform/...` pass on macOS.

## 4. Repository tools

- [x] 4.1 Delete the Windows code of `tools/ci`, `tools/release`, `tools/architecture` and `tools/coverage`, and
      collapse `tools/release/performance.ObserveCurrent` and `tools/ci`'s `supplyInUse` and
      `qualifyProtectedResource` (D4). Done when the package tests of those tools pass on macOS and no Go file under
      `tools/` holds a Windows code path, build constraint or test case.
- [x] 4.2 Re-derive the selection-matrix bound and `matrixJobMinutes` from the slowest remaining native job, citing
      its measured run, with tests that pin the derivation and not the number (D5). Done when the `tools/ci` tests
      pass.

## 5. CI declaration

- [x] 5.1 Declare one native job for macOS and one for Linux in `.config/ci/pipeline.cue`: GitHub on `macos-latest`
      and `ubuntu-24.04` with the default `bash`, GitLab on `macos-latest` with `review` or `protected` and on the
      unchanged Linux tag, and release runner choices `ubuntu-24.04`, `ubuntu-24.04-arm` and `macos-latest` (D2, D3);
      drop the Windows entries of `.config/ci/retained-steps.toml`. Done when `go run ./tools/ci project --check`
      passes on the regenerated projections and the `tools/ci/projection` tests pass.

## 6. Toolchain

- [x] 6.1 Restrict `.config/mise/mise.lock` to macOS ARM64, Linux ARM64 and Linux AMD64 through Mise's lock command,
      and drop the Windows-only asset patterns and templates of `.config/mise/config.toml` (D6). Done when the lock's
      diff removes rows only and changes no version, checksum or URL of a kept row.

## 7. Documents and records

- [x] 7.1 State the supported set once in the release policy, link to it from each document that names the
      supported platforms, remove Windows and Intel macOS from the current documents, and make `AGENTS.md`'s
      platform rule macOS and Linux with the per-forge runners of D2 (D7). Done when no current document names Windows or Intel macOS as supported and
      the prose and Markdown gates pass.
- [x] 7.2 Add a dated amendment line naming this Change to each decision record that names Windows as current
      behavior, and a breaking entry under `## Unreleased` in `CHANGELOG.md` (D7). Done when
      `go run ./tools/release validate-changelog` passes.

## 8. Local gates

- [ ] 8.1 Pass `mise run check` on the lane head, in the host queue.
- [ ] 8.2 Pass `mise run native` on the macOS host on the lane head, in the host queue.
- [ ] 8.3 A fresh review of the whole change by a reviewer who has not seen it. Done when its findings are answered
      in code or in the design.

## 9. Exact-HEAD proof

- [ ] 9.1 Pass the ETHOS full proof of the lane head.

## 10. Hosted CI

- [ ] 10.1 Pass the proposal's GitHub checks `Native macOS acceptance`, `Native Linux acceptance`,
      `Linux Secret Service` and `Quality and governance`, and read the macOS version and architecture from the
      first macOS job's log.
- [ ] 10.2 Pass the proposal's GitLab pipeline on the macOS runners tagged `macos-latest` and `review` and on the
      Linux runner.

## 11. Publication

- [ ] 11.1 Before the first `dev` push to GitHub, remove `Native Windows acceptance` from the required checks of
      `dev` and `main` (owner, 14:52), keeping the other check names. Done when both protections are read back.
- [ ] 11.2 Before the first GitLab push, obtain ETHOS 仓库纳管's confirmation that aigw-cli's macOS runners carry
      `macos-latest` with `review` and with `protected`.
- [ ] 11.3 After `main` is published on GitLab, ask ETHOS 仓库纳管 to drop the old macOS runner tags. Done when it
      confirms them dropped.
- [ ] 11.4 Tell ETHOS 仓库纳管 when this Change is on `dev` and `main` of both Forges, for the Windows guest step of
      its decision D54.

## 12. Housekeeping

- [ ] 12.1 Read GitLab's package registry for the Windows client supplier packages that the removed Windows job
      downloaded, and retire each one. Done when the registry lists none.
- [ ] 12.2 Retire `build/evidence/windows-removal-inventory-20261125.md` in the main checkout once the review of
      task 8.3 has used it. Done when the file is absent.
