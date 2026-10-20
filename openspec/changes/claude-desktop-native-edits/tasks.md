# Tasks

## 1. Desktop Profile Ownership

- [x] 1.1 Make Desktop tests expect that a profile re-saved by Claude Desktop
      is unchanged and keeps its bytes, that projections keep Desktop's own
      settings, that a changed managed setting refuses by name, and that the
      refreshed record keeps the earlier digest format; they fail before the
      change.
- [x] 1.2 Read the profile's managed meaning, accept equivalent saves, merge
      Desktop's keys into projections and name changed settings; the tests
      pass.
- [x] 1.3 A copy of the real profile that Claude Desktop 3.1623.0 saved is
      accepted without a profile write, and its refreshed record passes the
      earlier digest check.

## 2. Verification

- [ ] 2.1 `mise run check` and the full proof of the lane head pass.
