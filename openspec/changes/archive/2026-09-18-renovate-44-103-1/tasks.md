## 1. Refresh

- [x] 1.1 Replace Renovate 44.103.0 with stable 44.103.1 and its immutable multi-platform digest; verify the registry identity and exact declaration.
- [x] 1.2 Run the existing dependency and source gates and verify repeated native lock resolution is byte-clean.

## 2. Integrate

- [x] 2.1 Commit the signed result, obtain exact-HEAD proof, integrate it into local candidate, dev, and main, and publish it unchanged through each admitted Forge path.
- [x] 2.2 Verify exact-SHA hosted CI on both `dev` and `main` and remove the consumed proposal ref.
