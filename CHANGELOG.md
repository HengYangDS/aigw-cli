# Changelog

## Unreleased

- Add `aigw update --rollback` for portable installations: a local, reversible
  program-only swap with the single retained prior binary; native package
  channels remain owned by their package manager.
- Make portable installer and uninstaller help side-effect free on Unix and
  PowerShell, and retain exactly one immediately preceding portable binary for
  recovery across both archive installs and `aigw update`; portable uninstall
  removes that AIGW-owned rollback binary only.
- Retire historical GPT `-cdx` aliases: canonical Profile/model IDs no longer
  duplicate the Codex client scope, and validation plus residue gates reject
  their reintroduction.
- Keep `aigw check` anchored to the default Profile instead of allowing a
  client-specific override to silently replace the displayed current service.
- Redact known, URL-escaped, and bearer credentials before gateway or provider
  response text can reach diagnostics or errors.
- Make portable Unix installation self-contained under an empty or polluted
  `PATH`; the installer now bootstraps trusted system tool locations before it
  performs its local-only copy.
- Replace time-bound release snapshots with an evidence contract that separates
  local packaging, runtime installation, remote publication, and signed GA
  claims.
- Establish schema v2 as the single canonical structure for purpose-labelled team Profiles and local configuration.
- Add a hermetic portable-install lifecycle smoke test that proves install, ownership-scoped uninstall, configuration preservation, and source-binary preservation; run it in CI.
- Define evidence-gated Adapter admission for GLM/Z.AI, Gemini, Qwen, OpenCode, Perplexity, and Grok; unadmitted clients cannot be routed or placed in team templates.
- Centralize the static Claude/Codex admission registry so status, diagnostics, validation, and Adapter lists share one implemented-client boundary.
- Distinguish signed GA from checksummed RC delivery. CI explicitly blocks unsigned GA tags until protected macOS/Windows signing and notarization jobs are implemented and verified.

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
