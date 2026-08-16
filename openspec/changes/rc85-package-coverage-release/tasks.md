## 1. Release identity

- [x] 1.1 Set `VERSION` to `0.1.0-rc.85`.
- [x] 1.2 Add the rc.85 Changelog entry.

## 2. Source acceptance

- [x] 2.1 Validate the complete release Change with OpenSpec strict mode.
- [ ] 2.2 Execute the complete local proof on the final signed source HEAD.

## Delivery boundary

Hosted CI, independent Forge publication, release-asset installation,
runtime/provider acceptance, and Work Lane retirement are post-closeout
delivery effects. They remain required for the release but do not block
archiving the source Change that makes those effects possible.
