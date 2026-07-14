# 发布证据契约

本文定义**何种证据足以支持何种发布结论**。它不记录某个提交、RC 版本、分支、
GitLab 故障或签名身份快照；这些事实会变化，发布时应从当前工作区、CI 流水线、
GitLab Release 与签名工件中重新取得。

## 发布结论与所需证据

| 结论 | 所需的当前证据 | 不能替代它的证据 |
|---|---|---|
| 本地源码可打包 | 目标 revision 干净；`go test -race ./...`；`go vet ./...`；全部发布门禁 | 旧终端日志，或另一个提交的绿灯 |
| RC 工件矩阵完整 | `AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh <预发布版本> dist`；`check-release-artifacts.sh`；`test-release-package-layout.sh` | 部分压缩包，或没有校验和的构建成功 |
| 便携安装可用 | 针对候选二进制的 Unix 安装器与 PowerShell 安装器测试 | 仅静态审阅安装脚本 |
| Linux 包安装路径可用 | 在隔离网络的 Debian 与 RPM 系兼容镜像中，对 `amd64` 与 `arm64` 分别执行 `test-linux-native-install.sh dist <版本>`；或取得更强的原生发行版 runner 证据 | 交叉编译、压缩包检查，或镜像/容器失败、不可用时的旧结果 |
| Windows 安装器行为可用 | 在受管 Windows runner 上运行 PowerShell harness、`go test -race ./...` 和候选 `.exe`/`.cmd` 冒烟；PowerShell harness 是本机补充证据 | 仅检查 MSI 元数据、交叉编译，或非 Windows 主机上的 PowerShell |
| Release 已远端发布 | 对应 tag 的 GitLab package upload 与 Release job 成功；检查实际 Release 资产与校验和 | 本地存在 `dist/` 目录 |
| GA 已签名且可信 | 受保护 CI 对实际发布资产验证 macOS Developer ID + 公证/固化，以及 Windows Authenticode + 时间戳 | 未签名 RC、本机 identity 检查，或手工上传资产 |

任何本地检查通过，都不代表已经远端发布；任何远端上传，也不代表已签名 GA。发布结论
只能达到其对应的**当前证据强度**。

## RC 与 GA 的硬边界

`scripts/check-release-readiness.sh` 负责区分发布类别：

- `*-rc.*`、`*-beta.*`、`*-alpha.*` 可作为预发布打包，并以校验和和 SPDX SBOM
  作为证据；不得声称已签名或已公证。
- 无预发布后缀的版本，在受保护 CI 包含并验证全部生产签名流程前必须被阻断。不得用
  环境变量、手工上传或本地绕过跳过此门禁。

GA 的受保护 CI 必须对**同一组实际发布工件**证明：

1. macOS 二进制与安装包完成 Developer ID 签名、公证和 stapling，并有独立验证；
2. Windows 工件在受管 Windows runner 完成 Authenticode 签名与时间戳验证；
3. 签名后重新计算校验和，并在 publish/release 前重新验证；
4. 签名凭据只来自 GitLab protected/masked variables、runner keychain 或组织密钥服务。

## 跨平台安装证据

打包脚本生成 macOS/Linux/Windows 的 `amd64` 与 `arm64` 便携包、macOS Universal
`.pkg`、Linux `.deb`/`.rpm`、Windows `.msi`、校验和和 SPDX SBOM。
`check-release-artifacts.sh` 与 `test-release-package-layout.sh` 验证该矩阵的结构完整性。

结构证据与运行证据必须区分。Linux 验收脚本在 Debian 与 RPM 系兼容容器中，对
`amd64` 和 `arm64` 的 `.deb` 与 `.rpm` 分别执行离线安装并运行 `/usr/bin/aigw`。
容器在安装阶段使用 `--network none`；脚本会先将所需工件暂存到 Docker 可共享的
用户缓存目录，避免 macOS 容器运行时无法挂载私有 `/tmp`。这仍不等同于受管
Debian/Fedora 原生 runner 的证据，后者更强。若候选镜像尚未在本地缓存，镜像拉取
本身是外部前置条件；镜像可得后，工件安装与运行验收不依赖网络。若镜像、容器引擎
或网络不可用，应把 Linux 运行证据记录为“不可用”，不得复用旧结果。

## 发布步骤

1. 从目标的干净 revision 开始，记录该 revision SHA。
2. 运行完整验证套件，并以确切预发布版本打包。
3. 针对同一个 `dist/` 目录验证校验和、SBOM、包布局、安装器行为，以及当下可取得的
   Linux/Windows 运行证据。
4. 推送经审阅的分支，经受保护默认分支流程合入；随后从已合入提交创建预发布 tag。
5. CI 的 publish 与 release job 成功后，检查 GitLab 上**该 tag 的实际资产**；再从
   远端已发布工件做一次干净环境安装，方可称该 RC 可分发。
6. 只有上述受保护签名证据已对发布资产验证通过，才可创建 GA tag。

网络可达性、CI runner 容量、签名身份与 GitLab 状态都是运行时条件。发布时应即时
诊断和报告，不能把某次暂态结果写入本契约。

## Windows 原生验收最小集

Windows 支持不得由交叉编译或 macOS/Linux 的静态检查替代。稳定 Windows 产物前，受管
Windows runner 必须对同一候选工件至少证明：

1. Windows PowerShell 5.1 + ConsoleHost 无裸露 ANSI 控制序列；PowerShell 7 + Windows
   Terminal 保持可读输出。
2. `go test -race ./...` 与 `go vet ./...` 通过；Windows DACL 与 Credential Manager 的
   专项测试在真实 Windows API 上执行。
3. 便携安装、自定义安装、升级、回退和卸载可用；`.cmd` Claude shim 指向实际安装的
   `aigw.exe`，并能执行 `claude --version`。
4. Claude 最小验证在安全模式、显式模型和禁用本地扩展的边界内运行；成功、失败和超时
   都在上限内退出，且本次调用创建的后代不残留。

没有受管 Windows runner 时，这些项目应如实记录为待验收，不能将 PowerShell 脚本的
跨平台语法检查或 Windows 交叉编译标记为 Windows 原生通过。

仓库中的 `windows-native-acceptance` job 已定义这组验收，但只有组织注册真实 Windows
runner、赋予 `windows` tag，并将受保护变量 `AIGW_WINDOWS_NATIVE_RUNNER=true` 提供给
受保护 pipeline 后才会运行。该条件不能由 macOS runner、Wine 或交叉编译替代。
