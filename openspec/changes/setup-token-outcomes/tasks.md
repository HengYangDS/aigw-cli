## 1. Token Prompts

- [x] 1.1 Keep a provider's last verdict when the person enters the Token again and then presses Enter alone, or
      skips after text that is not a Token; a team setup test shows each reported as not accepted.
- [x] 1.2 Skip the provider when the answers to "What now?" name no choice; a test shows the team file applied, the
      other Token stored and the provider reported as not accepted.
- [x] 1.3 Say in a cancelled prompt's problem that no Token accepted in this run was stored, when one was; a test
      cancels the second prompt after the first Token was accepted, and a cancel before any Token keeps its wording.
- [x] 1.4 Read a provider that the command names more than once only once, and report a named provider skipped with
      Enter with the step that stores its Token; tests cover both, and a preview's row stays without a step.
- [x] 1.5 Say in a terminal preview that the run asks which providers' Tokens the person holds, when it would; a test
      covers a preview that would ask, one that names a provider and one without a terminal.
- [x] 1.6 Test `Prompt.OptionalSecret` in each of its forms: a value, Enter alone, input that ends, and a terminal
      read.

## 2. Shipped Team File

- [x] 2.1 Check from the file's structure that each client's recommendations name at most one model of a provider, in
      the file's provider order, and count a model as usable under any one of its protocols; the release test reads
      Codex's recommended family from the file and pins no provider count or URL.

## 3. Records And Verification

- [x] 3.1 Correct the rollout guide (setup contacts the provider for each Token it reads), the route evidence (the
      0.8.3 file supersedes the order of 2026-10-02), `README.md` and `CHANGELOG.md` (only providers without a Token
      are asked about) and the qualification record (GLM 5.2 was tried on Chat Completions alone; Desktop's depth;
      Suhe.AI lists no dated DeepSeek ID).
- [x] 3.2 State in `CHANGELOG.md` and the rollout guide that a provider whose model catalog answers any Token cannot
      have its Token proved by a validation, with the measurement of 2026-11-19 and the check that shows a wrong one.
- [ ] 3.3 Pass `mise run check` on the lane head.

## 4. Review Of The Token Flow

- [x] 4.1 Say what setup made of answers that named no choice, at the provider question and at "What now?"; a script
      drives the real prompts through a terminal: a refused Token left with such answers, then an accepted one.
- [x] 4.2 Cover the paths that the review traced without a test: the later of two verdicts, a cancel at the question
      after an accepted Token, text that is not a Token left with no choice, a preview with nothing to ask and with
      two named providers, and one provider named twice with piped input.
- [x] 4.3 Name in the records the check that shows a wrong Token (`aigw check` for a client that selects the
      provider's model, `aigw check <client> --model <provider/model>` otherwise), and state each scenario's
      precondition once.
