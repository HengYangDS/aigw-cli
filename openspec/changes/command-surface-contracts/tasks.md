## 1. Stage 1 — Contract fields

- [ ] 1.1 Add `terminal.Document`, which writes `schema_version` and `ok`, and give the failure envelope `schema_version` 1 and `ok: false`, plus `stage`, and `line` and `column` for a parse failure, when loading the configuration failed. A test decodes the envelope for each load phase and for an unrelated error, which has no `stage`.
- [ ] 1.2 Head the 16 existing JSON documents with `schema_version` 1 and `ok`: `status`, `check`, `doctor`, `use`, `sync`, `repair`, `setup --from`, `config import`, `config migrate`, `account list`, `account rename`, `route rename`, `route list`, `route show`, `catalog` and `installation`, which keeps its own `schema_version`. `ok` is true for a completed command and keeps its readiness meaning in `check` and `doctor`. The acceptance scripts' expected JSON gains only these fields.
- [ ] 1.3 Make `check` and `doctor` name an unreadable configuration's stage and position in JSON, as `status` and the human output do; an acceptance script covers a parse failure for each of the three, and each exits 1.
- [ ] 1.4 Make `cmd/aigw/commands_test.go` refuse a command script whose expected JSON document lacks an integer `schema_version` or a boolean `ok`; a test shows each refusal.
- [ ] 1.5 Record the two fields and the failure `stage` in CHANGELOG's `[Unreleased]` section.
- [ ] 1.6 Run the local quality graph at the stage head.
- [ ] 1.7 Prove the stage head with the full ETHOS proof.
- [ ] 1.8 Confirm the hosted three-platform graph and the real-client journeys pass at the stage head.

## 2. Stage 2 — JSON for every result-reporting command

- [ ] 2.1 Add `--json` to `add`, `route add`, `route edit`, `route remove`, `account edit` and `account diagnostics enable` and `disable`. Each document follows the converged rules at `schema_version` 1, and each command's acceptance script has a JSON success and a JSON refusal.
- [ ] 2.2 Add `--json` to `client discover`, `client enable` and `client disable` in the same form.
- [ ] 2.3 Add `--json` to `rotate`, `rollback`, `config path` and `setup` without `--from` in the same form. With `--json` none of them prompts; a Token comes only from `--token-stdin`, and a script shows the refusal without it.
- [ ] 2.4 Add `--json` to `test`, `verify`, `models` and `balance` in the same form; `verify --for all` reports each client's result and still writes the checkpoint only after every client succeeds.
- [ ] 2.5 Add `--json` to `install`, `update` and `uninstall` in the same form.
- [ ] 2.6 Make `cmd/aigw/commands_test.go` refuse a result-reporting command without `--json`, with `completion`, `config export` and `credential` as the declared foreign formats.
- [ ] 2.7 Run the local quality graph at the stage head.
- [ ] 2.8 Prove the stage head with the full ETHOS proof.
- [ ] 2.9 Confirm the hosted three-platform graph and the real-client journeys pass at the stage head.

## 3. Stage 3 — Previews and flag spellings

- [ ] 3.1 Add `--dry-run` to `setup`, `add`, `route add`, `route edit`, `route remove` and `account edit`. Each acceptance script previews against a private home and shows that the configuration, the credential store and client files are unchanged, and that no Token is read.
- [ ] 3.2 Add `--dry-run` to `client enable`, `client disable` and `rollback` in the same form.
- [ ] 3.3 Add `--dry-run` to `install`, `update` and `uninstall`. `update --dry-run` reads only public release metadata, and a test shows that it downloads no archive and changes no file or Keychain item.
- [ ] 3.4 Add `--upstream-model` beside `--model` to `setup` and `add`, and `--path` beside `--target` to `install` and `uninstall`. Both spellings bind one value, help shows the new spelling, and a test refuses both spellings together.
- [ ] 3.5 Run the local quality graph at the stage head.
- [ ] 3.6 Prove the stage head with the full ETHOS proof.
- [ ] 3.7 Confirm the hosted three-platform graph and the real-client journeys pass at the stage head.

## 4. Stage 4 — Convergence

Precondition: the product owner approves the converged contract in `design.md`.

- [ ] 4.1 Raise the 16 documents of task 1.2 to `schema_version` 2: references named after their type, `use`'s `model` and `upstream_model`, `selections`, one `projections` list, arrays instead of null, and snake_case values apart from configuration echoes. The acceptance scripts use a canonical Model that differs from its upstream ID.
- [ ] 4.2 Make `next_action` a complete runnable command or absent, with `next_detail` for a placeholder or prose, in every document, the failure envelope and the human Next section; doctor's `fix` becomes `next_action`. The contract test refuses a `next_action` that is not an `aigw` command or holds a placeholder.
- [ ] 4.3 Hide `setup --model`, `add --model`, `install --target` and `uninstall --target`, and warn on standard error with each replacement.
- [ ] 4.4 Make `test` without `--for` cover only the enabled clients; an acceptance script shows that a disabled client's endpoint is not probed.
- [ ] 4.5 Make `catalog --json` without configuration exit 1, a blocked rename preview write `ok: false` and exit 1, bare `aigw config` print its help and exit 0, and `aigw help <command>` print that command's help.
- [ ] 4.6 Move the release journeys' candidate steps to the converged flags and documents, keeping each baseline step on its release's surface.
- [ ] 4.7 Update README.md, the team rollout guide and the terminal experience document, and add CHANGELOG's table of every renamed flag and field and every changed default, value and exit status.
- [ ] 4.8 Run the local quality graph at the stage head.
- [ ] 4.9 Prove the stage head with the full ETHOS proof.
- [ ] 4.10 Confirm the hosted three-platform graph and the real-client journeys pass at the stage head.
