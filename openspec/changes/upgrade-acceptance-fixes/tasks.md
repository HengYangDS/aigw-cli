## 1. Upgrade acceptance

- [ ] 1.1 Make the first setup after an upgrade accept a Claude Code model
      preference: when no earlier selection exists and the model of the new
      selection reconstructs the recorded ownership digest, keep the user's
      preference, including its absence, and project the connection; a test
      reproduces the owner's settings of 2026-11-12 with `model` removed.
- [ ] 1.2 Make every refusal of an edited client file in a home without
      `aigw.toml` name a fix that runs there, never `aigw sync`; the
      fix-rerun command covers it in a script.

## 2. Team files

- [ ] 2.1 List every change that `setup --from` makes in its preview,
      including model labels that the team file resets; a test applies a
      team file twice after a local `model edit --label`.

## 3. Client verification

- [ ] 3.1 Start each native client of `aigw check --depth client` without a
      parent agent session's variables (`CLAUDECODE`, `CLAUDE_CODE_*`,
      `ANTHROPIC_DEFAULT_*` and their Codex and Hermes counterparts); a test
      runs the check with such variables set.

## 4. Help

- [ ] 4.1 Describe AIGW in root help without "routing", and extend the
      retired-word test to that word.

## 5. Supply

- [ ] 5.1 Take OpenSpec 2.0.0, pinned exactly, before 2026-11-12T22:41Z under
      the owner's release-age exception for that package; strict validation
      passes under it, `openspec update --dry-run` reports no change, and the
      full check passes.
