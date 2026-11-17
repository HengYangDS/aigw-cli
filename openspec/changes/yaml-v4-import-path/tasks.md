## 1. Pinned bytes

- [x] 1.1 Pin, under `go.yaml.in/yaml/v3`, the full bytes that AIGW writes into a Hermes configuration for each
      kind of content that the design names, and the full bytes of the ownership record for a mapping and a
      scalar model. Done when the tests pass at the base and a mutation of the writer's indentation fails them;
      v3 has no line width to mutate, so task 3.1 holds that mutation.

## 2. The record's own form

- [ ] 2.1 Store the ownership record's original model as a type of this repository with today's fields,
      converted to and from the library's node. Done when the pinned record bytes are unchanged and a record
      written by 0.8.1 reads.

## 3. The library

- [ ] 3.1 Import `go.yaml.in/yaml/v4` in place of v3 in every file, with the writer's line width and indentation
      stated. Done when the pinned bytes are unchanged, a line width of 80 fails them, `go.mod` names v4 alone,
      and the supply records name it.

## 4. Specification and release

- [ ] 4.1 The delta of `projection-format` validates strictly, and a simulated archive leaves the main
      specifications valid.
- [ ] 4.2 `VERSION` and `CHANGELOG.md` name 0.8.2.

## 5. Verification

- [ ] 5.1 `mise run check` passes on the lane head.
- [ ] 5.2 The hosted graph passes at the proposal head on both forges.
- [ ] 5.3 A fresh-context review of the change finds no byte that the move changes, or its findings are
      answered.
