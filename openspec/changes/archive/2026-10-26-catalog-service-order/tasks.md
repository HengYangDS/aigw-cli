## 1. Shared Ordering

- [x] 1.1 Add ranked, unranked, and multi-interface regressions that fail when
      Model order interleaves service groups.
- [x] 1.2 Repair `CatalogRouteIDs` through `CompareProtocols`; the regressions
      pass without changing canonical enumeration or exports.

## 2. Public Presentation

- [x] 2.1 Verify human and JSON Route lists share service grouping and preserve
      explicit bindings and stored bytes.

## 3. Source Acceptance

- [x] 3.1 Run affected package tests and the original formatting/validation
      commands with no regressions or unintended input changes.
- [x] 3.2 Pass the complete source gate and exact-HEAD proof of the signed
      source candidate.
