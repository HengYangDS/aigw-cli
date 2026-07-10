# Changelog

## 0.1.0

- Introduce Profile, Endpoint, inherited Route and isolated Adapter models.
- Store one secret per Profile in macOS Keychain, Linux Secret Service or Windows Credential Manager.
- Add Account + Runtime Profile model so multiple model choices share one Account Token.
- Add built-in model Profiles for gpt-5.6, gpt-5.5, gpt-5.5-ssvip, claude-sonnet, claude-opus and claude-fable.
- Add concise setup, add, use, rotate, status, test, doctor, balance and sync workflows.
- Add Claude process-boundary injection and AIGW-owned Codex provider projection, including optional model projection.
- Add secret-free team manifests and legacy local-prototype migration.
- Add portable archives plus native macOS pkg, Linux deb/rpm and Windows MSI artifacts for amd64 and arm64, checksums and SPDX SBOM.
