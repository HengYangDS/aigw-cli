## 1. Labels

- [x] 1.1 Derive a variant's label with the rest of its ID, and refuse two models of one provider with one label,
      naming both and a fix that works. Done when the domain, terminal, setup and model command tests cover a suffix, a
      prefix, an unrelated ID and a collision from a file, a team file and a command, and the team file loads.
- [x] 1.2 Label a family that `--family` creates with its own ID. Done when the model command tests show it.

## 2. Codex catalog

- [x] 2.1 Give the Codex alias the model's label without its provider as its display name, followed by its ID when
      the table shows that name, and no alias when it shows that too; the verifier checks the name AIGW writes. Done
      when the catalog tests pass and Codex 0.170.0 shows the alias under a name no other entry has.

## 3. Team file

- [x] 3.1 Make the curated-line test per provider, and replace Qwen 3.8 Max 0902 by Qwen4-Max on DMXAPI and AIHubMix.
      Done when the team-file tests pass and the qualification record names the three passing cells.

## 4. Verification

- [ ] 4.1 `mise run check` passes on the lane head.
