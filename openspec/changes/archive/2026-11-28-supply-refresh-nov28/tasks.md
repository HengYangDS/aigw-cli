## 1. Supply

- [x] 1.1 Advance uv, CUE, gh, glab and GoReleaser in the Mise configuration and regenerate its lock with `mise lock`.
      Done when `mise install --locked` installs each new release.
- [x] 1.2 Advance OpenSpec to 2.2.0 and markdownlint to 0.42.2 in the npm lock. Done when registry signatures verify
      and every Change validates strictly.
- [x] 1.3 Advance `github.com/charmbracelet/x/ansi` to v0.12.0. Done when `go mod tidy -diff` reports nothing and the
      packages that import it pass their tests.
- [x] 1.4 Advance the native clients to Codex 0.172.0, Claude Code 2.1.332 and Hermes v0.23.4 in the CUE authority.
      Done when `tools/ci project --check` reports no drift and the projection tests pass.
