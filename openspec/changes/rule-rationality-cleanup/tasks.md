## 1. Rule model

- [x] 1.1 Classify blocking invariants separately from review heuristics.
- [x] 1.2 Remove arbitrary structural threshold and ratchet policy fields.
- [x] 1.3 Retain positive topology, dependency, naming, portability, and public-surface contracts.

## 2. Implementation

- [x] 2.1 Delete private ELOC, complexity, nesting, directory, and suffix-group measurement code.
- [x] 2.2 Delete obsolete reports and threshold-focused tests.
- [x] 2.3 Add focused coverage for retained semantic naming and policy behavior.

## 3. Verification

- [x] 3.1 Pass the focused architecture test suite.
- [x] 3.2 Pass the complete repository source-quality graph without lowering its coverage floor.
- [ ] 3.3 Produce exact-HEAD ETHOS proof and integrate the atomic change.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:semantic structure` | `2.3` | `focused-architecture-tests-and-source-quality-graph` |
