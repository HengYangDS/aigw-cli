## 1. Dependency Authority

- [x] 1.1 Add the standard npm manifest, registry selection, and transitive lock for the current stable repository tools; verify a clean `npm ci --ignore-scripts` succeeds from an isolated cache and registry signatures verify.
- [x] 1.2 Remove npm tools from mise so each ecosystem has one version authority; verify the locked mise installation no longer resolves npm packages.

## 2. Shared CI Projection

- [x] 2.1 Make the CUE source jobs materialize the locked npm closure; verify the focused projection contract passes for GitHub and GitLab.
- [x] 2.2 Regenerate Forge workflows from CUE; verify projection drift checking passes.

## 3. Canonical Contract

- [x] 3.1 Correct contributor, release, and accepted-spec authority descriptions, including placeholders exposed by OpenSpec 1.11; verify OpenSpec strict validation and Markdown gates pass.
- [x] 3.2 Run focused Go tests, the source gate, and an isolated clean-cache installation; record exact commands and results before archive.
