## 1. Tests and rules

- [ ] 1.1 Bound the release tool's Keychain fixture calls by a named 30-second bound; `go test ./tools/release`
      passes on macOS and no five-second bound remains around `/usr/bin/security` in a test.
- [ ] 1.2 Make the team-file line test fail for an unclassified Claude, GPT or Gemini family; it fails with
      `gpt-6.2-astra-1120` beside `gpt-6-astra` and with `gpt-7`, and passes on the tracked file.
- [ ] 1.3 Read Hermes release tags with at most three major digits; the tag test rejects `v2026.9.24` and accepts
      `v0.23.1`.

## 2. macOS preflight

- [ ] 2.1 Remove a `mise` shell function, require `command -v mise` to name the verified executable and print the
      verified release in the preflight; the generated GitLab projection and the retained-step record follow.
- [ ] 2.2 Run the generated step in a macOS test against a stand-in executable; a match passes with `mise` bound to
      it, and a missing file, another digest and another version each exit non-zero.

## 3. Statements

- [ ] 3.1 Restate the pre-release candidate scenario of `release-distribution` and the shared macOS host scenario
      of `supply-chain`, and correct the rollout guide's sentence; strict OpenSpec validation of the Change passes.

## 4. Verification

- [ ] 4.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 4.2 The hosted graph passes at the proposal head on both peers, and a macOS job log shows the verified
      release line.
