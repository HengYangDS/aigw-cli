## Context

Direct declarations and repository tools are current. The native Go module
resolver remains the authority for compatible versions selected in `go.mod` and
`go.sum` for the repository's packages and tests.

## Decisions

- Use `go get -u ./...` followed by `go mod tidy`; never edit checksums manually.
- Accept only versions selected by the repository's locked Go toolchain.
- Repeat native resolution and require byte-identical module files. Do not turn
  update hints for modules outside the selected package-and-test graph into
  artificial direct requirements.
- Reuse the existing source, proof, native-platform, and Forge gates.

## Non-Goals

- Change product behavior or public contracts.
- Introduce another dependency manager or inventory.
- Force an incompatible major version.
