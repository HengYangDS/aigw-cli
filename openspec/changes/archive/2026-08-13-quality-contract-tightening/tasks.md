## 1. One quality graph

- [x] 1.1 Make the coverage, architecture, and gate registry files the only policy owners; delete duplicate thresholds and command bodies.
- [x] 1.2 Make one repository-native source command execute formatting, static analysis, architecture, coverage, governance, documentation, OpenSpec, build, and release-contract checks.
- [x] 1.3 Add contract tests proving local, ETHOS, GitLab, and GitHub projections select the same source graph without coupling their execution.
- [x] 1.4 Remove obsolete wrappers, compatibility paths, exclusions, aliases, and misleading evidence made redundant by the graph.

## 2. Quantitative coverage

- [x] 2.1 Add an independent branch-capable Go measure with raw numerator and denominator data for every package and the aggregate.
- [x] 2.2 Enforce statement and branch coverage strictly greater than 95 percent for every package and the module aggregate.
- [x] 2.3 Prove package completeness against `go list ./...`, exact package identity, exact source revision and tree, analyzer identity, and policy digest.

## 3. Semantic and physical structure

- [x] 3.1 Apply semantic topology, import direction, naming, file and directory size, function size, complexity, and nesting checks to production, test, and tool code.
- [x] 3.2 Split measured oversized or mixed-responsibility owners along semantic boundaries; delete forwarding-only owners and stale references immediately.
- [x] 3.3 Prove provider extension remains declarative and does not add command, projection, installer, lifecycle, or core dependency branches.

## 4. Portability and trust

- [x] 4.1 Run the same source contract natively on macOS, Linux, and Windows with repository-locked stable toolchain inputs.
- [x] 4.2 Verify structured commit messages and trusted signatures using external Forge-specific actor and trust inputs, with no product hard-coding.
- [x] 4.3 Verify docs, links, examples, environment-variable descriptions, CLI output, OpenSpec, build metadata, and release metadata against current product behavior.

## 5. Local proof

- [x] 5.1 Run focused tests for each repaired boundary, then the complete local source graph.
- [x] 5.2 Run full exact-HEAD ETHOS proof and record the attestation only after the candidate content is frozen.

## 6. Archive boundary

- [x] 6.1 Keep post-archive land, accepted-head CI, publication, installation, runtime acceptance, and retirement as distinct lifecycle evidence required by the canonical product-quality specification.

The Change ends here. Archive, land, accepted-head CI, independent Forge
publication, installation, runtime acceptance, and housekeeping occur after
this task list under the canonical specification and public ETHOS lifecycle;
they are not preconditions that make this Change impossible to archive.
