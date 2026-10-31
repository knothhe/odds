# 构建、安装、发布与更新

当前交付目标为 macOS 13+ / Apple Silicon（arm64），沿用 MyGo 0.7.3 / Onevra 原生 Go UI。版本唯一来源是根目录 `mygo.json`；构建写入包元数据，运行时直接读取。发布脚本已针对这个 JSON 版本源适配，没有新增第二份 VERSION。

## 本地命令

所有 Make 命令从仓库根目录执行：

| 命令 | 实际行为 |
| --- | --- |
| `make setup` | 创建仓库内 `.venv-build`，安装固定版本的 Dev 图标工具依赖 Pillow。 |
| `make signing-list` | 列出配对的代码签名身份及公开指纹、有效期，不导出私钥。 |
| `make signing-config IDENTITY=<SHA1>` | 将选定 Dev 身份引用保存在忽略的 `.odds-local.json`，以后复用。 |
| `make build` | 从当前源码构建固定证书签名的 `dist/dev/darwin-arm64/odds Dev.app`，不安装。 |
| `make install` | 重新构建并验签，通过暂存验证后安装到 `~/Applications/odds Dev.app`。 |
| `make uninstall` | 验证安装记录及实际 Dev 包，正常退出、清理该安装的登录项及应用登记，再移除程序；保留配置、数据和凭据。 |
| `make run` | 直接运行 Go 源码，使用 Dev 名称和数据目录。 |
| `make preview` | 使用临时配置运行；不注册全局快捷键、不更改登录项，退出后清理临时数据。鼠标动作仍真实执行。 |
| `make build CHANNEL=preview` | 显式生成不可安装的 ad-hoc QA 包，运行时强制使用临时预览模式。 |
| `make check` | Go 竞态测试、脚本回归测试，以及 MyGo 原生 UI 生命周期检查。 |

`INSTALL_DIR` 可指定同名 Dev 包的完整路径，例如 `make install INSTALL_DIR="/完整路径/odds Dev.app"`。卸载时传同一路径。忽略的 `.odds-install.json` 记录本 checkout 管理的确切安装及签名；换路径前先卸载原目标。未受管的手动复制安装需先核对 Dev 身份，再在原路径执行安装以建立记录；不手动伪造记录。符号链接、正式版、错误归属、未完成的登录项清理都会阻止删除。部分失败保留程序或记录供重试。

替换前先构建、验签及验证暂存包，再核对目标是否发生变化。正在运行的目标通过 macOS 原生正常退出请求关闭，等待设置保存；不会按模糊名称杀进程。替换失败恢复原包；恢复也失败时保留目标旁 `.odds-install-*/previous.app` 中的旧包，错误输出会给出保留位置和恢复命令，临时目录不会自动销毁。成功安装或成功回滚后清理暂存目录。

替换保留原登录启动登记；卸载才调用已安装的同一 Dev 程序，区分 SMAppService 的 NotRegistered / NotFound 与 Enabled / RequiresApproval，失败不冒充完成。安装记录分别保存登录项清理、LaunchServices 注销、目录删除阶段；删除中断时仅允许继续删除同一设备和 inode 的原目录。完成注销后不会再次扫描缺失的包，记录删除失败可直接重试。旧版记录若只确认登录项清理，则通过只读系统查询核对确切路径的应用登记；查询失败保留记录，其他路径的同 ID 副本不受影响。缺失包仍有旧登记时，按错误中的命令在原路径 `make install` 后再 `make uninstall`，不手动删除记录或全局重置 LaunchServices。

| 通道 | 应用名 / Bundle ID | 数据 | 更新资格 |
| --- | --- | --- | --- |
| Dev | `odds Dev` / `com.knothhe.odds.dev` | `~/Library/Application Support/odds Dev` | 不连接正式 Feed |
| 正式版 | `odds` / `com.knothhe.odds` | `~/Library/Application Support/odds` | 符合受控 CI 身份且安装位置可写时允许 |
| 预览 | Dev 预览名称，独立实例名 | 每次创建的临时目录 | 禁止正式更新及登录项修改 |

Dev 图标由正式图标派生，在右下角统一添加 `Dev`，正式资源保留。原始 `go tool mygo build` 可用于工具链排查；受控构建、安装和发布以 Make 入口为准。

## 固定代码签名与更新密钥

Dev 本地签名和正式发行签名分开选择。默认 Dev 不使用 ad-hoc；没有可用身份时构建和安装停止，运行 `make signing-list` / `make signing-config` 恢复。首次没有证书时，可在“钥匙串访问”→“证书助理”→“创建证书”选择 Self Signed Root / Code Signing，保留配对私钥后再明确选择。普通 build 不生成证书、不导出、不上传，也不添加系统信任根。

正式发行需要以下公开配置和 GitHub Secrets：

| 项目 | 内容 |
| --- | --- |
| `config/macos-signing.json` | 发行证书 DER 的 SHA-256 固定值，严格使用 `schema` 与 `certificateSHA256` 两个字段。 |
| `config/updates.json` | 已固定的 Ed25519 公钥、`knothhe/odds`、`v` tag 前缀及完整更新策略。 |
| `MACOS_SIGNING_CERTIFICATE` | 仅包含所选证书与配对私钥的 P12，Base64 编码后上传。 |
| `MACOS_SIGNING_CERTIFICATE_PASSWORD` | P12 随机导出口令。 |
| `MYGO_UPDATER_PRIVATE_KEY` | MyGo 更新包签名私钥，与公开配置中的公钥对应。 |

`make update-keygen` 在仓库外 `~/Library/Application Support/odds-signing/` 创建或复用本项目的更新密钥对，并只把公钥写进配置；拒绝覆盖已有公钥及经过符号链接的父路径。请离线备份私钥。`make signing-upload` 提供交互配置：显示目标仓库和 Secret 名称存在性，明确选择发行证书，通过 Security.framework 只导出该 `SecIdentity`，验证 P12 兼容导入及公钥对应关系，再上传到与 origin、公钥配置一致的仓库。导出口令通过 stdin 交给原生 helper，Secret 值通过 stdin 传给 gh，不进入源码和日志。可通过 `SIGNING_FLAGS=--dry-run` 验证而不上传；导出动作本身可能触发系统钥匙串授权。

配置上传入口不自动更换已有发行证书或更新公钥；冲突会停止。更换身份属于单独的迁移工作。Dev 证书可以与发行证书不同，不要求本机 Dev 配置 CI Secrets。

CI 将证书导入临时 keychain，核对唯一身份、公开证书 pin、包身份和 designated requirement，并在成功与失败路径恢复原搜索列表、删除私有临时材料。缺配置、导入或验签失败不会回退到 ad-hoc。自签名提供固定代码身份，不等同 Apple Developer ID 或 Apple 公证；若使用自签名发行，带 quarantine 的下载仍可能受到 Gatekeeper 限制。配置完整的 `APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD` 与适用的 Developer ID 后，CI 包装器可启用公证；三项须一起配置。

## GitHub Actions 发布

已配置 [.github/workflows/release.yml](../.github/workflows/release.yml)。正式构建与上传均由 Actions 执行；本地发布命令只做检查、版本、提交/tag 与推送。工作流使用固定的 action commit，macOS 15 arm64 构建、Ubuntu 24.04 发布；同一 tag 串行执行。源码检出精确 annotated tag，获取完整历史，校验 `mygo.json` 版本与 tag、源码提交和干净状态。

首次启用前，先完成发行签名配置，将公开配置与代码提交并推送 `main`。空的远程仓库尚无 main 时，需先推送分支；本地发布脚本不会代替这一步，也不会自动提交当前未完成的工作区改动。

```sh
make release-initial                         # 预览当前 0.1.0 的首次发布
make release-initial RELEASE_FLAGS=--apply    # 不改版本；创建 annotated tag 并推送

make release VERSION_BUMP=patch              # 预览下一 patch
make release VERSION_BUMP=patch RELEASE_FLAGS=--apply
# minor、major 或精确 SemVer 同样支持
```

首次发布不改版本、不创建版本提交；升级在检查通过后更新 `mygo.json`，创建版本提交与 annotated tag，再原子推送分支和该 tag。预览不运行构建、不写文件、不创建 tag 或 Release。工作区（包括未跟踪文件）必须干净；配置接入期间的预览被这一条件拒绝是预期行为。不要为了预览强行提交、丢弃或隐藏用户改动。

Actions 对两个发布入口执行同一条流程：检查 → 临时 keychain 导入 → 签名 app 与 DMG → Ed25519 签名最终 DMG → 包身份与最终 DMG 验证 → 上传受控 artifact → 发布 job 再验证 → 创建完整草稿并上传 → 核对远程内容 hash → 公开 Release。

后续版本的附件为（以 `0.1.1` 为例）：

```text
odds-0.1.1-arm64.dmg               # 手动安装与自动更新共用
update-darwin-arm64.json           # 版本、下载地址、大小和签名
SHA256SUMS                        # 上述两个文件的 SHA-256
```

正式构建仅生成一份 DMG，更新 manifest 的 URL、大小和 Ed25519 签名都对应这份最终 DMG，不生成或上传 tar.gz。沿用已锁定 MyGo 的更新界面和替换事务，通过受控补丁接入 DMG 签名与解包。macOS 构建 job 验证 DMG 签名、只读挂载，检查包内版本、源码 commit、CI 通道、架构与安全文件树，再复核代码签名。Linux 发布 job 复核同一受控 artifact 的 DMG 字节签名、manifest 和完整 SHA256SUMS；不在 Linux 上声称已挂载或检查应用包。

Release 正文保留 Full Changelog 及对应范围内的实际提交说明。签名包和全部附件均核实后才公开，避免用户读到不完整更新源。既有同名资产不自动覆盖，已公开 Release 在复验一致时保持原状。

## 失败与恢复

- 检查或构建失败时停止，不继续 tag/上传；保留现场。升级流程若已写版本但尚无提交，只在审核后还原本次版本改动再重试，不批量 reset/clean。
- tag 已创建而推送失败，使用精确版本续推，例如 `make release VERSION_BUMP=0.1.0 RELEASE_FLAGS="--resume --apply"`；不再次 bump，不移动 tag。
- tag 已推送时，重复推送同一 tag 通常不触发新的 workflow。先查看 `gh run list --repo knothhe/odds --workflow release.yml` 和原 run 日志；运行未结束时不要发起竞争发布。
- 构建成功而发布失败时，优先重跑失败的发布 job，复用同一 run 仍完整、可追溯的 artifact（保留 14 天）。已有部分草稿附件时不能以重新构建的同名文件覆盖它们；先核对内容并复用原产物补齐。
- 尚未上传附件且确需全量重建时，可显式 `gh workflow run release.yml --repo knothhe/odds -f tag=v0.1.0`。手动入口的 workflow 文件需已在默认分支。来源不明、产物过期或同名内容不一致时停止核查，不无限重跑。

## 自动更新的实际行为

正式版只在编译身份与包内记录一致、属于本项目 CI 构建、固定仓库与更新公钥齐全、目标为 arm64 且安装目录可写时注册 MyGo 原生 updater。Dev、临时预览、未受控原始打包及配置缺失的构建不会接入正式 Feed，也不会把“不支持”显示成“已是最新”。设置页与菜单栏均提供检查入口。

复用 MyGo 原生更新窗口及默认排期：启动后检查、运行中每 24 小时检查，唤醒时重新判断；最近成功检查及用户选择保存在正式数据目录的 `updater.json`。窗口展示当前候选的更新说明、检查错误、下载进度和重新启动入口。默认由用户选择安装；用户可在原生更新窗口明确选择后续自动下载/安装，重新启动仍由窗口操作。设置保存的退出屏障最多等待 3 秒，成功时不取消原生退出事件，因此保留更新器的重新启动请求；失败或超时保留输入并返回设置。

Feed 固定为 `https://github.com/knothhe/odds/releases/latest/download/update-darwin-arm64.json`，DMG 下载固定到相应版本的 Release。稳定 Feed 拒绝预发布候选；发布工具仍可制作预发布包。客户端将每个候选绑定到其独立 manifest；下载保留原生进度与取消，成功结束下载前用内置公钥验证 DMG 字节签名。安装阶段再次验签，再只读挂载隔离副本，核对包内真实版本、CI 正式身份、架构、commit 格式和安全文件树。仅复制 `odds.app` 到安装暂存目录，不执行 DMG 中的脚本或安装入口；原有 `/Applications` 链接和展示文件不会被复制。包内版本必须与候选完全一致，并严格高于当前版本；因此修改 Feed 版本号不能重放历史签名包。检查时同版本或旧版本的有效 Feed 仍可正常表示没有新版本。校验临时文件在关闭下载时清理，安装继续复用框架的替换逻辑。

受控构建应用 [MyGo 0.7.3 退出后安装脚本补丁](../patches/mygo-v0.7.3/install-after-exit.sh)，修复上游在新包移动与旧包恢复同时失败时删除备份的问题。补丁入口会核对依赖版本、两份原文件完整 SHA-256 和唯一替换位置；无法精确匹配、依赖被 replace 或已有其他 overlay 时停止构建。入口在私有临时目录复制依赖，仅修改副本中的安装脚本、DMG 下载解包入口与 DMG 签名逻辑，再通过临时 Go workspace 选择该副本。模块缓存及仓库的 `go.mod`、`go.work` 保持原样，临时文件随构建结束清理；升级 MyGo 时必须复核补丁。`make build/install/package`、Actions 正式打包及 `make run/preview` 均经过这一入口。Go 安装集成测试也通过同一入口编译临时 probe。

替换和旧包恢复都失败时，helper 保留安装位置旁 `.odds.app.update-*/old` 中的旧包、`app/odds.app` 中的新包和整个工作目录，将精确路径及恢复步骤写入 `RECOVERY.txt`，并停止自动启动。每次移动还核对目录的设备与 inode；若目标突然出现导致 macOS 把包移进其他目录，恢复记录会指出核验后的实际位置，避免误判成功并删除旧包。先退出相关实例并解决权限或文件系统错误，再核对该记录和目标路径后恢复；在确认恢复前不要删除工作目录。正常更新和成功恢复旧包时才清理暂存文件。该保护针对文件替换失败，不提供断电事务或启动健康检查保证。

公开下载与真实升级链路必须在首次 Release 成功后另外验收；本地 fixture、Dev 构建和 workflow 静态检查不等同已发布。

## Homebrew 分发

沿用现有公开仓库 `knothhe/homebrew-tap`，配方为 `Casks/odds.rb`，消费 odds 已公开的正式 DMG。支持范围仅为 macOS 13+ / Apple Silicon，不提供 CLI。配方只从已公开的 Release 生成，不使用占位 hash。

首次启用：为 **tap 仓库**配置可写 deploy key，将其私钥以 `HOMEBREW_TAP_DEPLOY_KEY` 存入 odds 仓库 Secret，并将 odds 仓库变量 `HOMEBREW_TAP` 设为 `knothhe/homebrew-tap`。此配置不在普通构建中使用。版本发布工作流成功公开稳定 Release 后，才传递确切 tag 调用 `homebrew.yml`；预发布不更新稳定 tap。未配置变量时跳过 tap 同步，不影响正式 Release。

同步下载并复核唯一 DMG 与 SHA256SUMS、可信 URL、uploaded 状态、正数大小和 hash，再生成配方。现有配方同版本同内容不提交；同版本内容变化、旧版本、脏 checkout 和未推送的无关提交会失败。源码与 tap 分开 checkout，tap job 串行、只提交目标配方、不 force push。并发冲突正常失败；查看 tap 新状态后通过同一入口有界重试。

本地可生成到 `dist/homebrew/Casks/odds.rb`，不改 tap：

```sh
make homebrew-cask RELEASE_TAG=v0.1.0
# 可选：在干净、main 分支且没有未推送提交的既有 tap checkout 中更新文件，不提交或推送
make homebrew-cask RELEASE_TAG=v0.1.0 HOMEBREW_TAP_DIR=/path/to/homebrew-tap
```

同步失败不改已公开 Release、不重新升版。可在 Actions 的 **Sync Homebrew cask** 手动入口传递原确切 tag 恢复；所有来源与版本校验仍生效。该 workflow 文件需先存在于默认分支。

配方公开并完成验收后，用户命令为：

```sh
brew install --cask knothhe/tap/odds
brew update
brew upgrade --cask --greedy knothhe/tap/odds
brew uninstall --cask knothhe/tap/odds
```

配方声明 `auto_updates true`，允许可写安装使用应用内 updater；Homebrew 升级需显式 `--greedy`。普通卸载退出准确 bundle ID，保留配置和数据；没有自动删除数据的 zap。保留 quarantine，固定签名不代替 Apple 公证；首次打开仍遵循当前签名和 Gatekeeper 的实际结果。

## 离线依赖信息

`make licenses` 从实际链接的 Go 包、精确模块版本及适用目录的 LICENSE/NOTICE 收集原文，同时收录 Go 工具链许可证。生成的 `source/internal/thirdparty/licenses.json` 进入源码、可执行文件嵌入资源和 `.app/Contents/Resources`。Onevra 改编组件与 Lucide 图标的独立许可证一并收录，现有 MyGo updater 补丁沿用上游授权文本。

`make check` 与打包均执行过期检查；升级依赖或工具链后重新生成并审阅。关于页离线按需展示，框架和模块版本优先读取实际构建信息；本地 replacement 明确标注，不展示临时构建路径。新增第三方位图、字体或其他非 Go 资源时，需要按其真实来源扩展收集范围，不能仅依赖模块许可证。

原生及公开链路的分层检查见[桌面验收](verification.md)。

## 界面语言

应用提供 `en` 与 `zh-Hans`，`CFBundleDevelopmentRegion` 为 `en`，bundle 声明这两种本地化。`config.json` 的 `language` 保存 `system`、`en` 或 `zh-Hans`，旧配置缺省为 `system`；未知偏好保留原值并显示英文。跟随系统使用 MyGo 的系统偏好解析与语言变更通知，无法匹配时回退英文；繁体中文不会误当作简体中文。

切换语言即时刷新设置、工具选项、快捷键标签、窗口标题、应用菜单与托盘菜单，保留当前分区、搜索和未保存修改。更新插件沿用同一应用语言；后续打开的更新对话框采用新语言，已在进行的对话框保持其创建时语言。框架控件与原生菜单加载 MyGo 的 `zh-Hans` 资源。系统拥有的 AppKit/WebKit 文案按 macOS 的应用语言机制在下次启动时采用新语言。所有应用文案集中在 `source/internal/uicopy/copy.go`，设计文案在 `design/copy.js`。
