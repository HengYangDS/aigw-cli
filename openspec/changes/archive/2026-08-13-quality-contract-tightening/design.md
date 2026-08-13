## Decision

Compile one repository quality graph from small native policy owners. The graph
is executed locally and projected unchanged into ETHOS proof and both Forge CI
planes. A green projection proves only its own execution; publication and
runtime acceptance remain separate evidence stages.

## Semantic Ownership

| Concern | Single owner | Responsibility |
| --- | --- | --- |
| Coverage | `.config/checks/coverage/policy.toml` | Measures statement and branch coverage and package completeness |
| Architecture | `.config/checks/architecture/policy.toml` | Declares semantic topology, dependency direction, size, complexity, naming, and portability |
| Gate graph | `.config/ci/verify-gates.toml` | Selects the repository commands required by every projection |
| Toolchain | `go.mod`, `mise.toml`, `mise.lock` | Declares language, tools, and reproducible resolved inputs |
| Source orchestration | `tools/ci` | Executes the declared graph without redefining policy |
| Repository validation | Cohesive packages under `tools/` | Implements only AIGW-specific checks behind narrow commands |
| Lifecycle evidence | ETHOS public commands | Binds execution to the exact repository head |
| Hosted verification | GitLab CI and GitHub Actions | Independently execute the repository graph |

## Quality Boundary

The boundary is positive and total: every tracked material belongs to an
applicable verification class. Go source includes production files, test files,
and repository tools. Repository material additionally includes CI, docs,
OpenSpec, build metadata, release metadata, installers, generated asset
contracts, and runtime acceptance fixtures. A new path is admitted by its
semantic class, not by extending a deny list.

Coverage has three independent assertions:

1. statement coverage is greater than 95 percent for every package and the
   module aggregate;
2. branch coverage is measured by an admitted branch-capable analyzer and is
   greater than 95 percent for every package and the module aggregate;
3. package completeness proves that every package in `go list ./...` appears
   exactly once in both quantitative results.

No value is inferred from another measure. Raw counts, package identity,
revision, tree, analyzer identity, and policy digest travel with the result.

## Dependency Direction

Product packages contain runtime behavior. Command packages assemble product
capabilities. Repository tools validate repository inputs and do not import
product runtime packages for convenience. Shared checks belong to the smallest
cohesive tool package; CI files call orchestration and contain no copied
implementation. Tests follow the owner they verify and may use dedicated
fixture packages only where reuse is behavioral rather than forwarding.

## Cross-platform Execution

The source graph is shell-independent. macOS, Linux, and Windows invoke the
same Go entry point through the locked toolchain. Platform-native jobs prove
native behavior; cross-compilation proves only artifact construction. GitLab
and GitHub consume the same repository contract but publish independently.

## Completion Model

Completion is conjunctive and staged:

1. focused tests close the measured gap;
2. the complete local source graph passes;
3. exact-HEAD ETHOS proof passes;
4. native hosted CI passes independently on both Forges;
5. release assets and checksums agree across independent publications;
6. a clean install and runtime acceptance pass;
7. absorbed lanes and obsolete surfaces are retired.

Evidence from an earlier stage never substitutes for a later one.

## Rejected

- treating Go statement profiles as branch evidence;
- excluding tests or tools from structural checks;
- duplicating commands or thresholds in CI provider files;
- adding wrappers, aliases, compatibility readers, or a second build system;
- hard-coding a user, machine, checkout, signer, credential, provider, or Forge;
- coupling one Forge's verification or publication to the other Forge;
- retaining obsolete policy or evidence as a regression-prevention mechanism.

## Requirement To Task To Proof

| Requirement | Tasks | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.2` | `repository-source-graph-and-projection-contracts` |
| `product-quality:faithful quantitative quality evidence` | `2.2` | `per-package-and-aggregate-statement-branch-package-reports` |
| `product-quality:bounded semantic structure` | `3.1` | `production-test-and-tool-architecture-report` |
| `product-quality:portable repository text` | `4.1` | `native-macos-linux-windows-source-gates` |
| `product-quality:actor-independent contribution policy` | `4.2` | `explicit-trust-provenance-reports` |
| `product-quality:complete delivery evidence` | `5.2` | `exact-head-ci-release-install-runtime-retirement-receipts` |
