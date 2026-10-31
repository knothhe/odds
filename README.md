# odds

odds 是一个 macOS 菜单栏小工具集合（odds and ends，零碎小工具）。用 Go 编写，界面用 [MyGo](https://github.com/egoist/mygo) 的原生 UI（GPU 绘制，不使用 WebView），组件来自 [Onevra](https://github.com/oneo-me/onevra)。

正式版下载见 [GitHub Releases](https://github.com/knothhe/odds/releases)，支持 macOS 13+ / Apple Silicon。

## 工具

### 鼠标居中

| 动作 | 默认快捷键 |
| --- | --- |
| 移到当前屏幕中央 | ⌃⇧⌘C |
| 移到下一块屏幕中央（多显示器循环） | ⌃⇧⌘N |
| 高亮鼠标位置（不移动） | 无 |

移动后会在指针处显示一个收缩的圆环，可在设置中关闭或换颜色。移动指针使用 `CGWarpMouseCursorPosition`，全局快捷键使用 Carbon `RegisterEventHotKey`，都**不需要**辅助功能权限。

快捷键在「设置 → 鼠标居中」中修改：点击快捷键框后按下新组合键；Esc 取消，Delete 清除。组合键须含 ⌃、⌥ 或 ⌘ 之一（F1–F12 可单独使用）。

## 开发

需要 Go 1.27.3+（`GOTOOLCHAIN=auto` 会自动下载）。Go 代码在 `source/` 下，根目录的 `go.work` 引用了它，打包配置 `mygo.json` 在根目录：

```sh
make setup               # Dev 图标工具；首次运行一次
make signing-list        # 查看可用签名身份
make signing-config IDENTITY="证书的SHA1"  # 明确选择并记住本机 Dev 证书
make build               # 固定签名的独立 Dev 应用
make install             # 从源码构建并安装到 ~/Applications/odds Dev.app
make uninstall           # 卸载同一 Dev 安装，保留配置和数据
make preview             # 临时配置预览，不注册全局快捷键或更改登录项
make check               # 竞态测试、脚本测试与 MyGo 原生 UI 检查
make verify-native       # 隔离原生窗口验收，不改登录项或全局快捷键
make licenses            # 依赖升级后重新生成离线许可证
```

Dev 与正式版的名称、bundle ID、数据目录和更新资格分别隔离。构建产物位于 `dist/dev/darwin-arm64/odds Dev.app`；正式包交由 GitHub Actions 从精确版本 tag 构建。首次发布用 `make release-initial`，后续用 `make release VERSION_BUMP=patch`，均默认只预览；实际执行需明确传 `RELEASE_FLAGS=--apply`。签名、安装路径、Secrets、附件、首次与升级发布及失败恢复见 [构建与发布说明](docs/build-release.md)。

正式版接入 MyGo 原生自动更新窗口，默认每 24 小时检查，可从设置页或菜单手动检查；更新包使用固定公钥验证。Dev 和临时预览不会连接正式更新源。

预览退出后会清理临时配置；预览中的鼠标动作仍会真实执行。正常运行的设置由单个后台写入器原子保存，连续修改合并为最新快照。保存失败时保留本次修改并提供「重试保存」；退出会等待正在进行的保存，失败时可重试或明确选择「放弃并退出」。无法读取的设置文件会保留原件，本次修改不会写回。

版本以 `mygo.json` 为来源，MyGo 打包时注入，设置界面直接读取；普通 `go run` 显示「开发构建」。

设置格式使用 schema 1，兼容原来的无版本文件；兼容格式的未知字段会保留。遇到较新格式时保留原文件并进入只读模式，提示升级应用。关于区域支持复制当前版本、打开项目主页，以及离线查看依赖版本与许可证；构建和 `make check` 会检查清单是否过期。

Homebrew 已准备好针对现有 `knothhe/homebrew-tap` 的生成与同步流程；维护者启用同步并公开配方后，可使用 `brew install --cask knothhe/tap/odds`。启用条件、升级与卸载命令见[分发说明](docs/build-release.md#homebrew-分发)，原生与发布后检查见[验收说明](docs/verification.md)。

应用图标的源文件是 `assets/icon.svg`，修改后重新导出 PNG：

```sh
swift scripts/svg2png.swift assets/icon.svg assets/icon.png 1024
```

正式版设置保存在 `~/Library/Application Support/odds/config.json`；Dev 使用独立的 `odds Dev` 目录。

## 界面设计

打开 [设计画布](design/index.html) 查看通用、鼠标居中、快捷键录制、保存失败、退出受阻、只读保护，以及深色与窄窗口状态。后续可见界面改动先更新相关画板，再实现到现有 MyGo / Onevra 组件，并在实际原生窗口中核对；状态和验证约定见 [设计说明](design/README.md)。

## 目录结构

```text
.
├── mygo.json                     # 打包配置
├── go.work                       # 引用 source/ 模块
├── assets/                       # 应用图标等资产
├── design/                       # 设置界面设计画板与状态说明
├── scripts/                      # 构建、安装、签名与发布入口
├── docs/                         # 发布与更新说明
└── source/
    ├── main.go
    └── internal/
        ├── app/                  # 菜单栏、设置窗口、配置、全局快捷键
        ├── tool/                 # Tool 接口
        ├── hotkey/               # 快捷键录制控件与格式转换
        ├── cursor/               # 移动鼠标指针（macOS）
        └── tools/
            └── mousecenter/      # 鼠标居中
```

## 添加新工具

1. 在 `source/internal/tools/<name>/` 中实现 `tool.Tool`：
   - `Actions()` 返回可绑定快捷键的动作，`ID` 以工具 ID 为前缀且长期不变；
   - `Settings()` 返回可 JSON 序列化的设置指针（没有设置则返回 `nil`）；
   - `SettingsItems(changed)` 构建设置页中的选项行，修改后调用 `changed()` 保存。
2. 在 `source/internal/app/app.go` 的 `registry()` 中加入它。

菜单栏菜单、设置页侧栏、快捷键的注册、冲突检测与保存都会自动处理。
