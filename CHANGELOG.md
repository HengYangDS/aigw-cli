# Changelog

## 0.1.0

- Introduce Profile, Endpoint, inherited Route and isolated Adapter models.
- Store one Account-scoped secret in macOS Keychain, Linux Secret Service or Windows Credential Manager; multiple model Profiles inherit it without duplicate copies.
- Add Account + Runtime Profile model so multiple model choices share one Account Token.
- Add built-in model Profiles for gpt-5.6-sol-cdx, gpt-5.5, gpt-5.5-ssvip, claude-sonnet-5, claude-opus-4-8-thinking and claude-fable-5.
- Add concise setup, add, use, rotate, status, test, doctor, balance and sync workflows.
- Add Claude process-boundary injection and AIGW-owned Codex provider projection, including optional model projection.
- Add strict real-response verification, secret-free full-verification checkpoints, and lifecycle-free configuration rollback.
- Add secret-free team manifests and legacy local-prototype migration.
- Add portable archives plus native macOS pkg, Linux deb/rpm and Windows MSI artifacts for amd64 and arm64, checksums and SPDX SBOM.
- Ensure native package workflows never traverse user homes or delete user-level Claude shims.
- Move Unix Claude shims from shared user bins into AIGW-owned data directories, with a reversible secret-free shell PATH activation block and owned-legacy migration.
