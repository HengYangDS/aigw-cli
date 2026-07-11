# Changelog

## 0.1.0

- Introduce Profile, Endpoint, inherited Route and isolated Adapter models.
- Store one Account-scoped secret in macOS Keychain, Linux Secret Service or Windows Credential Manager; multiple model Profiles inherit it without duplicate copies.
- Add Account + Runtime Profile model so multiple model choices share one Account Token.
- Remove implicit provider/model defaults from first-run setup; example model Profiles now live only in secret-free team manifests.
- Isolate provider-native exact diagnostics behind explicit Provider Diagnostics integrations, while keeping generic Account health checks available for every service.
- Add concise setup, add, use, rotate, status, test, doctor, balance and sync workflows.
- Add Claude process-boundary injection and AIGW-owned Codex provider projection, including optional model projection.
- Add strict real-response verification, secret-free full-verification checkpoints, and lifecycle-free configuration rollback.
- Add secret-free team manifests.
- Add portable archives plus native macOS pkg, Linux deb/rpm and Windows MSI artifacts for amd64 and arm64, checksums and SPDX SBOM.
- Ensure native package workflows never traverse user homes or delete user-level Claude shims.
- Use AIGW-owned Unix Claude shim directories with a reversible secret-free shell PATH activation block.
