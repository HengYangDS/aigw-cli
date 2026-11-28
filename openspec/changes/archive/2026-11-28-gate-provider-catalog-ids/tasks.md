## 1. Provider IDs

- [x] 1.1 Make the profile test identify the behaviour gates by `provider.code-behavior`. Done when the test fails
      against the current profile.
- [x] 1.2 Declare `provider.code-behavior` for `go-behavior` and `node-behavior` and `provider.code-static` for
      `go-static-analysis` in `.ethos/profile.toml`. Done when the profile test passes and every provider that the
      profile declares is an ID of the runtime's provider catalog and none is in its legacy table, the condition on
      which ETHOS reports `gate_provider_legacy_name`.
