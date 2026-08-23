## 1. Contract and failing tests

- [x] 1.1 Define the Profile-owned native-provider contract and validate the Change strictly
- [x] 1.2 Add failing configuration tests for defaulting, persistence, replacement, and validation
- [x] 1.3 Add failing Codex projection tests for attribution, idempotence, transition, removal, and invalid commands
- [x] 1.4 Add failing synchronization tests for projection and authentication behavior

## 2. Product implementation

- [x] 2.1 Add the narrow Profile and Runtime provider fields without Account fallback or aliases
- [x] 2.2 Generalize the existing attributed Codex projection to one exact provider identity
- [x] 2.3 Bind the installed AIGW credential command and skip generic login/catalogue for native providers
- [x] 2.4 Update canonical documentation and changelog

## 3. Proof and closeout

- [x] 3.1 Run focused tests and the affected static checks
- [x] 3.2 Run the complete locked source gate once on the final candidate
- [ ] 3.3 Archive the Change, integrate the signed object, and verify both Forge projections
- [ ] 3.4 Retire the absorbed stale proposal refs and the completed Work Lane
