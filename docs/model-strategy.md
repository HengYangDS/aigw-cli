# 模型策略与客户端准入

## 目标

使用者不应面对数百个模型 ID，也不应为每个新模型理解一套新配置。AIGW
采用**少量能力席位**：每个日常场景只保留一个首选项；模型、Account、Token 与
客户端适配始终各归其位。

- **Account** 是一个上游服务商账户、端点和一把系统密钥。
- **Profile** 是一个可运行的 `Account + 已验证客户端 + 模型` 选择。
- **purpose** 是 Profile 的一行人类提示；它帮助选择，不参与路由、重试或密钥管理。
- **Catalog** 是只读发现结果，绝不是推荐集，更不会自动创建 Profile。

因此，运行 `aigw use` 时看到的是少量明确用途的 Profile，而不是网关返回的完整
模型目录。`aigw catalog --all` 仍然完整保留透明度，供维护者做准入判断。

## 当前能力席位

下表是产品的精简推荐，不是 AIGW 内置的服务商默认值，也不代表所有模型已配置或
已适配。具体模型 ID 必须在对应 Account 的目录、权限、协议测试和使用成本均通过后
才可成为本机 Profile。

| 能力席位 | 首选候选 | 使用边界 | 当前状态 |
|---|---|---|---|
| 默认 Agent | `claude-fable-5` | 通用 Agent、长链路工作 | 已由 Claude Adapter 支持；Claude 默认基线 |
| Codex 工程 | `gpt-5.6-terra-cdx` | Codex 内的代码与工程任务 | 已由 Codex Adapter 支持；Codex 默认基线 |
| 深度推理（按需） | `claude-opus-4-8-thinking` | 只有默认 Agent 不足时才显式切换 | Claude Adapter 支持；不作默认 |
| 平衡备选（按需） | `claude-sonnet-5` | 明确需要更轻量的 Claude 路径时 | Claude Adapter 支持；不作默认 |
| 中文高阶代码 / Agent | `glm-5.2-cc` | 中文复杂代码与 Agent 对照 | 待独立 GLM/OpenCode Adapter 准入 |
| 长上下文与多模态 | `gemini-3.1-pro-preview` | 大文档、图像与超长上下文 | 待独立 Gemini Adapter 准入 |
| 中文日常性价比 | Qwen 3.5 Plus（以 Account 目录中的精确 ID 为准） | 日常中文通用任务 | 待独立 Qwen Client Adapter 准入 |
| 外部检索研究 | `perplexity-deep-research-ssvip` | 需要联网检索、引文与交叉核验的研究 | 待研究 Client/协议准入；绝不作 Claude/Codex 默认 |
| 交叉核验备用 | `grok-4.5` | 观点探索与独立复核 | 不进入日常推荐或默认路由 |

`DeepSeek`、`Kimi` 与 `MiniMax` 不在推荐集、团队模板或默认路由中。它们可能仍在
某个上游的只读模型目录里出现；AIGW 不隐瞒目录，也不会因发现结果自动创建或推荐
Profile。

## 已运行模板

团队的最小模板只保留当前已有客户端适配证据的四个 Profile：

```text
Claude Fable 5      默认 Agent
GPT-5.6 Terra Codex Codex 代码与工程
Claude Opus         复杂推理（按需）
Claude Sonnet       平衡备选（按需）
```

维护者可在清单中以 `purpose` 提供同样的选择提示：

```toml
[profiles."claude-fable-5"]
label = "Claude Fable 5"
purpose = "默认 Agent"
account = "team-gateway"
client = "claude"

[profiles."claude-fable-5".models]
claude = "claude-fable-5"
```

对于当前还没有客户端 Adapter 的能力席位，不要把模型伪装成 `claude` 或 `codex`
Profile，也不要复用 Claude/Codex 的配置目录、Token 映射或路由。先完成适配，再让
它出现在可选 Profile 中。

每个候选的协议证据、专属配置边界、真实验证与回退门槛见 [Adapter 准入](adapter-admission.md)。

## 准入门槛

一个新客户端或模型进入推荐集，必须按以下顺序通过，而不是“目录里看得到”就加入：

1. **客户端边界**：定义专属 Adapter、配置位置和卸载边界；不得写入 Claude/Codex
   的目录，也不得覆盖其认证。
2. **协议证据**：对该客户端真实使用的协议验证认证、模型选择、流式响应、工具调用
   与必要的多模态/上下文能力；OpenAI-compatible 不自动等价于 Codex Responses。
3. **密钥边界**：新服务商使用独立 Account 与一把系统 Token；不复制现有 Account 的
   Token，不把 Token 写入团队清单或客户端配置。
4. **最小真实验证**：在明确允许消耗额度后运行有上限的真实请求；成功后才写入不含
   密钥的验证检查点，并确保 `aigw rollback` 可回到上一个已验证状态。
5. **推荐决策**：比较任务效果、稳定性、成本、区域可达性和维护负担；每个能力席位
   只保留一个首选项，其他只可作为明确的备用项。

任何一步失败，AIGW 保持现有路由和客户端可用，不创建半配置 Profile，不启动、停止
或重启桌面客户端。

## 使用者路径

成员日常只需记住：

```bash
aigw                 # 看当前 Profile 与两条客户端路由
aigw use             # 从带用途提示的少量 Profile 中选择
aigw check           # 判断 Token、额度、模型、网络或本地配置问题
```

维护者在接入一个已经通过准入的模型时才需要：

```bash
aigw profile add <profile> \
  --account <account> --for <client> --model <model-id> \
  --purpose "一行用途提示"
```

这保持了“少量选择、完整能力、严格边界”：使用者无需理解服务商细节，维护者也不会
把未经验证的模型或客户端路径带入生产路由。
