# odds

odds 是一个 macOS 菜单栏小工具集合（odds and ends，零碎小工具）。用 Go 编写，界面用 [MyGo](https://github.com/egoist/mygo) 的原生 UI（GPU 绘制，不使用 WebView），组件来自 [Onevra](https://github.com/oneo-me/onevra)。

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

需要 Go 1.27.3+（`GOTOOLCHAIN=auto` 会自动下载）。Go 相关的一切都在 `source/` 下：

```sh
cd source
go run .                 # 直接运行
go tool mygo dev         # 热重载开发
go test ./...
go tool mygo build       # 打包为 ../dist/darwin-arm64/odds.app 和 .dmg
```

应用图标的源文件是 `source/assets/icon.svg`，修改后重新导出 PNG：

```sh
swift scripts/svg2png.swift source/assets/icon.svg source/assets/icon.png 1024
```

设置保存在 `~/Library/Application Support/odds/config.json`。

## 目录结构

```text
source/
├── main.go
├── mygo.json                     # 打包配置
└── internal/
    ├── app/                      # 菜单栏、设置窗口、配置、全局快捷键
    ├── tool/                     # Tool 接口
    ├── hotkey/                   # 快捷键录制控件与格式转换
    ├── cursor/                   # 移动鼠标指针（macOS）
    └── tools/
        └── mousecenter/          # 鼠标居中
```

## 添加新工具

1. 在 `source/internal/tools/<name>/` 中实现 `tool.Tool`：
   - `Actions()` 返回可绑定快捷键的动作，`ID` 以工具 ID 为前缀且长期不变；
   - `Settings()` 返回可 JSON 序列化的设置指针（没有设置则返回 `nil`）；
   - `SettingsItems(changed)` 构建设置页中的选项行，修改后调用 `changed()` 保存。
2. 在 `source/internal/app/app.go` 的 `registry()` 中加入它。

菜单栏菜单、设置页侧栏、快捷键的注册、冲突检测与保存都会自动处理。
