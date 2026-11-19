## 1. Prompt And Probe

- [x] 1.1 Add `MultiSelect` to `invocation.Prompter` and implement it in `terminal.Prompt`, with a numbered
      accessible form that reads space- or comma-separated numbers and takes Enter alone as no choice; prompt tests
      cover a choice, no choice, an answer out of range and input that ends.
- [x] 1.2 Make the probe's failures name the provider: a refusal by its label and status, another status or no answer
      by the endpoint's protocol and provider; probe, provider and invocation tests show each wording.
- [x] 1.3 Add `OptionalSecret` to `invocation.Prompter`, a hidden prompt that returns nothing for Enter alone, in both
      forms of `terminal.Prompt`.

## 2. Team Setup

- [x] 2.1 Ask, in a terminal without `--provider`, which of the team file's providers without a Token the person has,
      and prompt only for those; a team setup test answers with one provider and with none.
- [x] 2.2 Validate each Token as it is read; in a terminal let Enter alone skip a provider, and offer to enter a Token
      again or skip its provider after text that is not a Token or a failed validation, and to try the same Token
      again after no verdict; tests cover a refusal entered again and skipped, Enter alone, malformed text, an
      accessible question that names nothing, and a 503 tried again and skipped.
- [x] 2.3 Apply the team file and the accepted Tokens whatever the other Tokens' outcome, keep a Token that the store
      held, report a provider whose Token was not stored as `rejected` or `unverified` with its reason and next step in
      schema version 4, and exit 1 after the write when a piped Token was not stored, showing its problem in place of
      the success line; tests cover the human and the JSON report and a held Token.
- [x] 2.4 Reproduce the reported case end to end: the shipped team file with only an AIHubMix Token configures every
      client that AIHubMix serves, and Enter at the provider question applies the file with no Token.

## 3. Shipped Team File

- [x] 3.1 Recommend, for each client, a model on every provider that serves the client's first recommended family; a
      store test reads that coverage from the file's structure.
- [x] 3.2 Write a team file export's array wider than 100 columns one element to a line, as Taplo does; an encoder test
      covers a wide, a narrow, a quoted and an unbalanced value, and the shipped file passes both the export layout and
      Taplo.

## 4. Records And Verification

- [x] 4.1 Record the breaking changes and the fix in `CHANGELOG.md`, naming every new JSON field and state.
- [x] 4.2 Describe the provider question, skipping and a piped Token that is not accepted in `README.md` and the team
      rollout guide.
- [x] 4.3 Pass `mise run check` on the lane head.
