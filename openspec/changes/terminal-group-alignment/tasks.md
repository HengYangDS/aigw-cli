# Tasks

## 1. Renderer

- [x] 1.1 Make presentation tests expect one value column per group from its
      longest label and state symbol with at least two cells of separation,
      values and details continued under that column, whole-group stacking
      below 20 value cells, four-cell notes at every width, a hanging success
      statement and space-only wrapping; they fail before the change.
- [x] 1.2 Give `Field` its state and details, lay out `Rows` by group, remove
      `Row` and `Status`, and wrap at spaces; the presentation tests pass.

## 2. Commands

- [x] 2.1 Make a doctor test expect the checks of different label lengths in
      one value column with their fix details under it, help tests expect
      option lines at the section indentation, and a verify test expect each
      client's details under its row; they fail before the change.
- [x] 2.2 Submit every command renderer's rows as groups, give the sync
      preview's credential entrypoint, Client Bindings and projection targets
      their own sections, end previews with a success line, print verify
      details after their client row, and indent help options with their
      section; the command tests pass.
- [x] 2.3 Record the change in `CHANGELOG.md` and the layout guidance in
      `docs/experience/terminal-experience.md`.

## 3. Verification

- [x] 3.1 Point the dependency-acceptance fixture at braces 3.0.2 and
      GHSA-grv7-fg5c-xmjg, because OSV withdrew GHSA-vfj7-8cjw-p6xm for braces
      3.0.3 on October 23; the network gate passes.
- [x] 3.2 `mise run check` and the full proof of the lane head pass.
