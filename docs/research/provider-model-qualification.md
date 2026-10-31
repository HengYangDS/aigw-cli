---
subject: aigw:provider-model-qualification
role: research
state: active
relations: {}
---

# Provider Model Qualification Evidence

These observations inform the October 27, 2026 team manifest; they neither
maintain a live catalog nor override the [Route inventory](../../manifests/team.toml).
For setup/selection, use the [team rollout guide](../guides/team-rollout.md#reviewed-model-defaults).
Listings, bounded inference and native-client tool loops prove distinct claims.

## Curated model choices

Select one general model per vendor from exact Account/protocol Routes qualified
by inference and native-client evidence; positioning, suffixes, listings and vendor
benchmarks inform selection but do not prove independent quality rankings.

The October 1, 2026 primary-source review closes four missing rationales:

- Google's Gemini 3.8 Flash, selected over 3.1 Pro Preview on October 5, is replaced by the [October 16 admission](#october-16-gemini-4-argon-admission).
- Z.ai's [GLM 5.3 model card](https://huggingface.co/zai-org/GLM-5.3)
  reports coding/long-horizon post-training improvements over GLM 5.2, supporting
  general coding use. Vendor-reported cross-model scores are not AIGW benchmarks.
- Moonshot's [Kimi K3 source](https://github.com/MoonshotAI/Kimi-K3) called K3 its most
  capable model; the [October 27 refresh](#october-27-model-refresh) replaces it with K3.1 Pro.
- Qwen's [3.8 model card](https://huggingface.co/Qwen/Qwen3.8-2.4T-A95B) calls
  Qwen3.8-Max its managed flagship; the October 27 refresh pins its 0902 snapshot.

The October 4, 2026 review did not admit speculative replacements. Anthropic's
[Fable page](https://www.anthropic.com/claude/fable) still identified Fable 5.1,
without a verified Fable 5.5 API identity, and Gemini 4 Argon was trusted-tester-only.
MiniMax's [language models](https://platform.minimax.io/docs/guides/models-intro) identified
M3 for coding; [H3](https://www.minimax.io/blog/minimax-h3) generates video and
sound, not text-agent inference. Step 5 Preview was discovery-only at the
September 25 read; its later [admission](#october-56-model-refresh) supersedes that observation.

Public catalogs returned 417 AIHubMix and 128 UCloud IDs; anonymous UCloud omitted
GPT/Claude, while DMXAPI required authentication. Neither omission nor anonymous
401/disabled-pricing 403 proves model absence. Bindings, credentials and models were unchanged.

Stable tool admission does not make inference models GA. Qualified general-agent
Previews may remain explicitly named, non-default Routes; unqualified ones remain
discovery-only. Preserve this distinction and explicit local selections.

## October 27 model refresh

`aigw catalog --json` read all eight authenticated surfaces at 11:46 +08:00
(SHA-256 `e4dbc199…`). Each candidate was judged by its vendor's own release
notes, not by its identifier, then qualified per Account and protocol by an
isolated Hermes `read_file` turn and a tool-free resumed recall, both bounded.
The runs used the operator's patched Hermes v0.22.5+1.g57fd07f (the last retry v0.22.5+2.g9f06c16); every
admitted Route then passed once more on the stock v0.22.5 (`bf0203a7`) that CI pins. Raw results are in the canonical checkout's `build/evidence/team-model-refresh-20261027/`.

| Model                    | Admitted Account / protocol                 | Vendor basis                                                 |
| ------------------------ | ------------------------------------------- | ------------------------------------------------------------ |
| GPT-6.1 Terra            | AIHubMix, DMXAPI CDX / Responses            | OpenAI models page; joins the GPT series                     |
| DeepSeek V4.1 Flash 1017 | all three / Responses                       | October 17 change log: retrained, TB4 31.2 → 42.3            |
| Kimi K3.1 Pro            | DMXAPI, UCloud / Responses; AIHubMix / Chat | Moonshot: stronger long-horizon coding, fewer hallucinations |
| Qwen 3.8 Max 0902        | AIHubMix, DMXAPI / Responses                | Model Studio: an upgraded snapshot of qwen3.8-max            |
| MiniMax M3.1 Flash       | all three / Responses                       | model card: builds on and improves M3                        |
| Muse Wonder              | all three / Responses                       | Meta: its most capable models                                |
| Solar Pro 4              | AIHubMix / Chat                             | Upstage flagship; its withdrawn channel serves again         |

Kimi K3.1 Pro timed out twice on DMXAPI before passing, and AIHubMix's Responses
endpoint returned HTTP 404 for it. UCloud lists only the undated Qwen ID, whose
build it does not document, so Qwen leaves UCloud. Doubao Seed 2.1 Pro 260915
(DMXAPI) returned HTTP 429 burst protection three times, and Mistral Large 4
and Command A+ (AIHubMix) returned HTTP 400 three times; their predecessors stay.
Nemotron 3.5 Super and 3 Ultra both passed; NVIDIA keeps Ultra for the highest
reasoning accuracy, so Ultra stays. iFlytek and Microsoft were left out by decision.

## October 16–19 Claude Route observations

On October 19, Claude Desktop 2.42304.0 (Claude Code 2.1.299) ran its host shape
on UCloud with experimental betas off: `ToolSearch` loaded deferred MCP tools
that completed on all three Claude Routes, Code, Chat and Cowork completed tool
calls and Code a compaction, so these Routes now declare `tool_search`.

The October 18 cache-continuity step found that this workstation's October 11–18
Claude client requests on `claude-opus-5-5` read the cache in 92.6% of 52,214 pairs.
The 14,245 answered with a `msg_` ID and `inference_geo: "global"` hit in 79.5%, a
cost limitation (80.1% between two such responses), against 96.3–99.5% for five
other signatures. Five scripted journeys each on UCloud `claude-sonnet-5-5` and
`claude-haiku-5-5`, one signature per journey, hit in 19 of 20 pairs (the miss on
`msg_bdrk_`) and 20 of 20; both are too few to rank signatures. UCloud's guidance
asks for a stable `metadata.user_id`, which Claude Code 2.1.302 already sends.

## October 16 Gemini 4 Argon admission

Google's [changelog](https://ai.google.dev/gemini-api/docs/changelog) made its flagship reasoning model
[`gemini-4-argon`](https://ai.google.dev/gemini-api/docs/models/gemini-4-argon) stable on October 13. AIGW
0.4.2 then listed the exact ID on all eight authenticated surfaces, after one failed AIHubMix read.
Hermes v0.22.2 (local `2cde9522`, modified tree), using existing credential commands, read a private
file via `read_file` and recalled it tool-free in the resumed session on AIHubMix and direct DMXAPI
Responses and UCloud Chat (37.9/15.2, 60.5/11.8, 14.3/16.8 s). An earlier AIHubMix run stalled for
210 s; every run warned `install out of sync`. Argon replaces Gemini 3.8 Flash; other clients stay unqualified.

## October 10 Haiku 5.5 admission

Eight authenticated observations list `claude-haiku-5-5` at AIHubMix/UCloud and
`claude-haiku-5-5-cc` at DMXAPI, with 420/277/528 IDs respectively. This supersedes
only the dated Haiku absence, not the Fable 5.5 or Gemini 4 limits. The October 10
catalog withdraws Fable 5.1 and Astra/Luna 6 under the Claude 5.5+/GPT 6.1+
floors. No qualified Fable, Astra, Terra, or Luna successor was observed.

Claude Code 2.1.296 at `xhigh` passed exact text, `Read` and same-process recall
for all three Routes; the second input omitted the marker. Probes used original
readers, private scratch and no session persistence; user state stayed unchanged.
Opus 5.5/Sol 6.1 recommendations remain; raw results bind `cc4ddf9ac8c264fc29795ae885964143a7959c3c` under the existing verification owner.

AIHubMix rejected `context_management` with HTTP 400; UCloud passed text but
rejected tool continuation with a duplicate-`tool_result` Bedrock error of
unproved origin. Both then passed in isolated [compatibility mode](../guides/team-rollout.md#client-compatibility)
with `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`. Retain failures and this condition;
default betas, other clients/platforms, full-window and release bytes remain unqualified.

## October 5–6 model refresh

Eight sources returned 417/523/275 AIHubMix/DMXAPI/UCloud IDs, unchanged on October 6.
Dated [model overview](https://platform.claude.com/docs/en/models/overview) and
[release notes](https://platform.claude.com/docs/en/release-notes/overview) reads established neither Fable
nor Haiku 5.5, Gemini 4 stayed trusted-tester-only, and [MiniMax M3.1 Flash Preview](https://platform.minimax.io/docs/guides/models-intro)
required M Plan and appeared in no Account. Installed AIGW 0.3.1 passed native Claude/Codex
checks for UCloud Opus 5.5/Sol 6.1 with configuration byte-identical.

The following direct channels completed text inference, an actual Hermes
`read_file` call, a final marker and same-session recall with medium effort:

| Model               | Qualified Account / protocol                               | Selection rationale                                                                                                                                                              |
| ------------------- | ---------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Gemini 3.8 Flash    | AIHubMix and DMXAPI / Responses; UCloud / Chat Completions | Google's [current GA workhorse](https://deepmind.google/models/gemini/flash/) targets coding and agents.                                                                         |
| Step 5 Preview      | AIHubMix / Chat Completions; DMXAPI / Responses            | StepFun's [flagship agent release](https://www.stepfun.com/step-5-preview) replaces the earlier Step 3.7 option.                                                                 |
| HY4 Preview         | DMXAPI / Responses                                         | Tencent's [successor announcement](https://www.tencent.com/tencent-releases-and-open-sources-tencent-hy4-preview/) identifies improvements over HY3 for real productivity tasks. |
| LongCat 2.5 Preview | DMXAPI / Responses, exact wire `LongCat-2.5-Preview`       | Meituan's [September 25 release](https://longcat.chat/platform/docs/change-log) adds multimodal understanding and stronger coding.                                               |

The three Preview labels remain explicit, non-default choices; vendor claims are
not independent benchmarks. UCloud Gemini 3.8 Responses returned `convert_request_failed`
(its Chat Route passed), AIHubMix HY4 exceeded the bounded completion deadline, and
Command A+ returned `no_available_channel`. DeepSeek [phased out V4 Pro](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)
in favor of V4.1 Flash, so the older Pro name was not restored.

The isolated Hermes was 0.21.5, upstream `af90026a`, local `75e983671a840a1adcf249ab524d3c7847105489`,
with OpenAI SDK 2.24.0; this proves that client's direct journey only. DMXAPI's
operator-selected local Responses Proxy rejected these native inputs with
`unproved provider-portable structure` while the direct channels passed; do not
infer Proxy acceptance from direct-provider success.

Import retains explicit selected Routes, credentials and endpoint overrides; retire
an unselected older Route only through native import retirement after checking its
projections and recovery consumer. The retained-state import then passed with 26
Models and 58 Routes and preserved Account endpoints, client selections and
credential commands; installed AIGW 0.3.1 still reported Hermes drift because its
checker compared logical IDs.

## Provider catalog and route evidence

[Provider catalog and route evidence](provider-catalog-route-evidence.md)
records the September 30 to October 9 catalog reads and bounded inference that
qualified the shipped Routes.

## Codex native chooser

Codex can execute a non-OpenAI Responses Route without listing it in its
[native `/model` chooser](https://developers.openai.com/codex/cli/slash-commands).
On 2026-09-27, an isolated Codex CLI 0.157.1 session ran Grok 4.7 but its chooser
listed only bundled GPT models. Separately, the AIGW-projected Grok Route completed
an authenticated tool loop against a controlled endpoint. Select such Routes with
[`aigw use --for codex`](../../README.md#use-it-every-day) and verify them through
Codex; AIGW does not alter an existing Desktop conversation's model.

Codex 0.160 still omits Grok 4.7 from its bundled metadata. Its controlled native
tool loop succeeds but emits a fallback-metadata warning. Public `aigw verify`
therefore reports incomplete native qualification and does not update a successful
checkpoint; AIGW invents no entry to silence Codex. Functional inference, native
metadata, the chooser, and current upstream availability are separate claims.
On 2026-10-14, official Codex 0.162.1 passed real-Provider `aigw verify` through
the signed 0.4.0 candidate on the UCloud GPT-6.1 Sol Route, with native file tools
and same-session recall, but emitted that warning on UCloud DeepSeek V4.1 Flash.

On October 4, 2026, the installed official Codex 0.160.0 bundled catalog contained
11 models. The team manifest then had 37 Responses Routes: nine matched native wire
IDs, three DMXAPI Routes required canonical alias projection, and 25 Routes across
11 non-GPT Models lacked native metadata, 11 on AIHubMix and seven each on DMXAPI
and UCloud. Client acceptance remains Route-specific, including the recorded Sol 6.1
CDX acceptance. The selected GPT-6.1 Sol entry existed in both the native and
preserved user catalogs. This identity audit does not qualify live inference or
authorize replacing the user's catalog. The earlier fourteen-Route observation
described a provider's complete test scope, not the current missing metadata count.

Every Claude Route in the [shipped team manifest](../../manifests/team.toml)
currently declares only an Anthropic interface, whereas Codex's
[custom-provider contract](https://learn.chatgpt.com/docs/config-file/config-reference#configtoml)
supports the Responses wire API. No shipped AIGW Claude Route is therefore
admitted for Codex. A provider-native Responses endpoint or an explicitly
selected external adapter would need real Codex text, tool, streaming, and
replay qualification before such a Route could be offered. AIGW does not
translate inference traffic or configure ordinary ChatGPT conversations.

## Windows native sandbox boundary

The October 4, 2026 original Codex 0.160 Windows journey passed four retained-
predecessor stages and fourteen Route outcomes, but its sandboxed PowerShell
child exited `0xC0000142`. A same-SID observer ran in Session 0 on a service
window station; it could not open `WinSta0`. This is an execution-context
observation, not proof of the actual Codex child's desktop.

The exact upstream [desktop implementation](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/windows-sandbox-rs/src/desktop.rs)
creates a private desktop in the caller's current station but specifies
`Winsta0` in the child startup name. The [process implementation](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/windows-sandbox-rs/src/process.rs)
explicitly connects an invalid startup desktop to PowerShell's
`STATUS_DLL_INIT_FAILED`. Open [issue #46412](https://github.com/openai/codex/issues/46412)
reports the same Session 0 versus interactive-session split on earlier Codex
versions. Together they support a station-mismatch hypothesis; they do not
establish that every Windows failure has that cause.

Keep the original failed result and [native private-desktop isolation](https://learn.chatgpt.com/docs/windows/windows-sandbox).
Qualification requires the original tool loop in a supported native Windows
context, or a released vendor repair. Disabling the private desktop, widening
ACLs, replacing PowerShell, or substituting WSL would prove a different boundary.
Ordinary source CI does not silently enable real-client acceptance; an explicitly
selected `--clients` release journey must still pass the complete native suite.
