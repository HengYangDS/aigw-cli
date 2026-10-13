# Tasks

## 1. Retained Item Migration

- [x] 1.1 Add a failing private-keychain regression. Today the worker is denied
      an item written by `/usr/bin/security` with its default access list. The
      worker must return that item's Token, leave the exact slot AIGW-owned (its
      decrypt list names the worker, not `/usr/bin/security`) and leave no
      staging item.
- [x] 1.2 Migrate on an eligible denial under the per-user lock, with staging
      and verification. Cover an interrupted migration from each persisted state
      and concurrent first reads. Also cover ineligible items (a locked keychain,
      or an item that trusts no application or another one): they must keep their
      denial without invoking `/usr/bin/security`.
- [x] 1.3 Migrate a denied retained item before a write or deletion, and remove
      stale staging items on every write and deletion.
- [x] 1.4 Before `aigw update --rollback` activates the retained program, convert
      the configured Accounts' items back to `/usr/bin/security` ownership, and
      refuse activation if a conversion fails. Keep the conversion a no-op
      outside macOS.
- [x] 1.5 Record the change in the Unreleased Changelog and in DR-0011, and pass
      strict official OpenSpec validation and an archive rehearsal.

## 2. Local Gates

- [x] 2.1 Vet and lint the darwin, linux and windows targets, and run
      `mise run check` and `mise run native` once on the stable tree.

## Workflow follow-up

- Announce the landing to the `terminal-product-convergence` holder, then prove,
  land, archive and close out through ETHOS.
- Publish GitHub `dev` and `main` through a proposal pull request after its
  required checks pass. Publish GitLab when it is reachable again.
- With the operator's explicit confirmation, run the signed candidate once
  against one real retained item and convert it back, before the operator
  upgrades.
