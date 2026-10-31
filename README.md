# odds

macOS 菜单栏小工具集合，使用 Go 和 [MyGo](https://github.com/egoist/mygo) 原生 UI 构建。

支持 macOS 13+ / Apple Silicon，从 [GitHub Releases](https://github.com/knothhe/odds/releases) 下载。

## 鼠标居中

| 动作 | 默认快捷键 |
| --- | --- |
| 移到当前屏幕中央 | ⌃⇧⌘C |
| 移到下一块屏幕中央 | ⌃⇧⌘N |
| 高亮鼠标位置 | 无 |

可在设置中修改快捷键、关闭或调整高亮效果，无需辅助功能权限。

## 开发

需要 Go 1.27.3+（`GOTOOLCHAIN=auto` 可自动下载），代码位于 `source/`。

```sh
make setup                              # 首次准备构建工具
make signing-list                       # 查看可用签名身份
make signing-config IDENTITY="证书的SHA1" # 配置本机 Dev 签名
make install                            # 构建并安装到 ~/Applications/odds Dev.app
make preview                            # 使用临时配置预览
make check                              # 运行测试与原生 UI 检查
make uninstall                          # 卸载 Dev 应用，保留数据
```

Dev 与正式版的应用和数据相互隔离。预览不注册全局快捷键或更改登录项，鼠标动作仍会真实执行。

- [构建与发布](docs/build-release.md)：签名、打包、自动更新与 Homebrew 分发。
- [验收说明](docs/verification.md)：原生窗口与发布后检查。
- [界面设计](design/README.md)：设计画板与界面改动流程。
