## 1. Module Boundary

- [x] 1.1 Declare `ignore ./build` in `go.mod`. With Go files from two
      packages under `build/`, `go vet ./...` and `go list ./...` fail without
      the directive and pass with it, both in ETHOS's environment
      (`GOTOOLCHAIN=local`, `GOPROXY=off`).

## 2. Refresh

- [x] 2.1 Calculate candidates with the pinned Renovate image (`dryRun=full` on
      GitHub `dev` at `0438dd9b`), then check each candidate's identity and
      release (October 26: five admissible updates, two held).
- [x] 2.2 Advance gh, the Renovate image and mise-action in their declarations,
      regenerate gh's lock entries and the GitHub projection, and confirm that
      `tools/ci project --check` passes.

## 3. Verification

- [x] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (`0c6a5746`, statement coverage 95.07%).
- [x] 3.2 The full proof of the lane head passes (Attestation `2480f661`).
