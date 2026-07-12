# Release Readiness

## 当前状态（2026-07-12）

本机已通过代码、安装 smoke、残留和 15 工件打包验证；这只证明源代码和未签名 RC
工件可构建，**不等同于远端发布或正式 GA 交付**。

受控检查的结果：

| 项目 | 结果 | 含义 |
|---|---|---|
| macOS `codesign` identity | 0 个有效 identity | 本机不能签署 Developer ID 工件 |
| `xcrun notarytool` | 可用 | 有工具，但没有组织认证 profile/identity |
| GitLab SSH | 10 秒超时 | 不能推送、建 MR、合并或读取远端分支 |
| GitLab API | 5 秒连接超时 | 不能创建 Release 或上传工件 |

因此当前分支只能形成**本地已验证、待远端发布**的状态。

## 已完成的真实验收（2026-07-12）

以下验收均基于已签名提交 `e082b00`，而不是包含其他并发未提交改动的工作区；
每次真实模型请求均经用户明确授权。

| 范围 | 结果 | 证据边界 |
|---|---|---|
| macOS 主账户 | Claude 与 Codex 真实最小请求通过 | `verify --for all`；checkpoint 为 schema v2，且不含密钥 |
| macOS 隔离账户（UID 502） | Claude 与 Codex 真实最小请求通过 | 临时 HOME、临时 AIGW shim、Token 仅在子进程环境中存在，退出后清除 |
| Linux ARM64 容器 | Claude 与 Codex 真实最小请求通过 | 临时容器、只读配置挂载、只读 secret 挂载；配置不含密钥 |
| Linux ARM64 `.deb` | 安装并执行通过 | 容器内 `dpkg -i` 后从 `/usr/bin/aigw` 执行 |
| Linux AMD64 `.deb` / `.rpm` | 工件 payload 执行通过 | 从包中提取 `/usr/bin/aigw`，在 `linux/amd64` 容器内执行 |
| 交付矩阵 | 15 个 RC 工件完整 | `check-release-artifacts.sh`、checksums 与 SPDX SBOM 均通过 |

Linux AMD64 容器基础镜像因 Docker Hub 超时不可拉取，因此没有声称完成
`dpkg` 系统级安装；已用同一包 payload 在 AMD64 容器实际执行补足二进制和包内容证据。
RPM 已验证格式、payload 路径和二进制可执行性；其系统级安装仍应在具备 RPM 发行版
基础镜像或受管 runner 后补做。

最新一次受控推送尝试使用 `ssh -o ConnectTimeout=10` 向
`codex/initial-product` 推送，SSH `192.168.64.101:1122` 超时。该尝试未改变远端；
在网络恢复前不再重试。当前不能创建 MR、合入 `main`、打远端 tag 或发布 Release。

## RC 与 GA 的硬边界

`scripts/check-release-readiness.sh` 的规则是：

- `*-rc.*`、`*-beta.*`、`*-alpha.*`：允许使用校验和与 SBOM 作为 RC 交付证据；仍不得声称已签名或已公证。
- 无预发布后缀的 GA 版本：CI 在 package 之前 fail-closed。不会上传、创建或标记一个未签名的正式 Release。

禁止用环境变量、手工上传或跳过 CI 来绕过 GA gate。

## GA 所需的受保护 CI 能力

解除 GA gate 前，必须在受管 runner 中完成并验证：

1. macOS：Developer ID 对二进制和 pkg 签名，notarytool 公证，stapler 固化，随后独立验证签名与公证状态。
2. Windows：在受管 Windows runner 对 exe/MSI 执行 Authenticode 签名，并独立校验签名链和时间戳。
3. 发布前：对签名后的全部工件重新计算校验和；CI 验证每个签名资产，随后才允许 publish/release。
4. 凭据：仅可使用 GitLab protected/masked variables、runner keychain 或组织密钥服务；不得放入仓库、AIGW 配置、团队 manifest 或 shell history。

届时应把真实签名作业加入本仓库，并以签名验证命令替代当前的 GA 阻断脚本；不能仅设置一个“已签名”布尔变量。

## 远端恢复后的顺序

```bash
git push -u origin codex/initial-product
# 在 GitLab 建立 MR，CI 通过后合入 main
# 从 main 创建 rc tag；RC 发布后执行干净环境安装验证
```

只有组织签名流水线完成后，才创建无预发布后缀的 GA tag。每次真实模型验证还需要用户
明确同意消耗相应 Account 的额度；它与发布凭据、签名身份完全独立。
