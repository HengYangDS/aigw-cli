# Tasks

## 1. Project Qualified Tool Search

- [x] 1.1 Make Route validation tests expect `tool_search` on the Anthropic
      interface and reject it on the others; they fail before the capability
      exists.
- [x] 1.2 Add the `tool_search` Route capability; the validation tests pass.
- [x] 1.3 Make Desktop profile and Adapter tests expect `toolSearchEnabled`
      only when every exposed Route declares `tool_search`, its drift refusal
      and its change detection; they fail before the projection exists.
- [x] 1.4 Project the key from the exposed Routes' capabilities; the tests
      pass.
- [x] 1.5 Record the contract and the Desktop tool-search and cache-continuity
      qualification steps in the docs within their length limits, and add the
      change to `CHANGELOG.md`.

## 2. Verification

- [x] 2.1 `mise run check` passes, and the lane head is proved.

## 3. Qualification and Release

- [ ] 3.1 Run the cache-continuity step on the UCloud Claude Routes and record
      the consecutive-request hit rate of each response signature.
- [ ] 3.2 Release the projection with the next AIGW release through the
      existing release workflow, with the operator present for notarization.
- [ ] 3.3 With the released build, declare `tool_search` on each UCloud Claude
      Route in local configuration and run the Desktop tool-search journey:
      MCP tools listed by name, one deferred tool call, a multi-turn tool
      continuation and a compaction in Code, and one tool call each in Chat
      and Cowork; record the app, bundled Claude Code and Route. Until 3.4
      ships, a team import sees that local declaration as a changed Route.
- [ ] 3.4 Declare `tool_search` in `manifests/team.toml` on the Routes that
      passed 3.3, with a team manifest test, and release that catalogue.
