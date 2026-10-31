package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"

	"github.com/knothhe/odds/source/internal/hotkey"
	"github.com/knothhe/odds/source/internal/tool"
)

func (a *App) openSettings() {
	if a.quitting {
		return
	}
	if a.settings == nil {
		title := Name + " 设置"
		if a.preview {
			title += " · 预览"
		}
		shell := plain.NewShell(plain.ShellOptions{
			Title: title, Width: 760, Height: 560, MinWidth: 480, MinHeight: 420, StateKey: "settings",
			Hidden: true, Theme: theme.Onevra, Content: a.settingsView,
		})
		a.settings = shell.Window()
		a.settings.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.recorder.Stop()
			a.settings.Hide()
		})
		// Recording ends when the window loses the keyboard.
		a.settings.OnBlur(a.recorder.Stop)
		a.settings.OnHide(a.recorder.Stop)
	}
	if a.packaged && !a.preview {
		a.loginEnabled = mygo.App.OpenAtLogin()
	}
	if a.settings.IsMinimized() {
		a.settings.Restore()
	}
	a.settings.Show()
	mygo.App.Focus()
	a.settings.Focus()
}

func (a *App) settingsView(c *ui.Context) {
	if a.recorder.Active() == "" && a.settings != nil {
		// While a shortcut is recorded, ⌘W goes to the recorder.
		c.OnShortcut(ui.Cmd, ui.KeyW, a.settings.Close)
	}
	a.recorder.BeginFrame()
	defer a.recorder.EndFrame()

	categories := []components.SettingsCategory{a.generalCategory()}
	for _, t := range a.tools {
		categories = append(categories, a.toolCategory(t))
	}
	w, _ := c.Size()
	ui.Column(c).Fill().MinWidth(0).Gap(0).Children(func() {
		a.settingsNotice(c)
		components.Settings(c, &a.settingsState, categories, components.SettingsOptions{
			Narrow: w < 620, Title: "设置", SearchLabel: "搜索设置", SearchPlaceholder: "搜索设置",
			CategoryLabel: "设置分类", ResultsTitle: "搜索结果", EmptyText: "没有匹配的设置",
			HeaderActions: func(c *ui.Context) {
				components.Text(c, a.settingsStatus()).FontSize(c.Theme().Rem(.8)).TextColor(c.Theme().TextMuted).Role(ui.RoleStatus)
			},
		})
	})
}

func (a *App) settingsStatus() string {
	switch {
	case a.readOnly:
		return "修改仅本次有效"
	case a.saving:
		return "正在保存…"
	case a.saveErr != "":
		return "尚未保存"
	case a.recorder.Active() != "":
		return "正在录制快捷键"
	case a.preview:
		return "预览 · 使用临时设置"
	default:
		return "更改自动保存"
	}
}

// Notices live outside the searchable catalog so navigating or filtering never
// hides a persistence failure. Saving keeps the current input and runtime state.
func (a *App) settingsNotice(c *ui.Context) {
	title, detail := "", ""
	switch {
	case a.readOnly:
		title = "设置文件不可用"
		detail = "原文件已保留；本次修改不会保存。请检查文件访问权限后重新启动应用。"
	case a.saveErr != "":
		title = "设置未保存"
		detail = "本次修改已保留，退出前请重试保存。 " + a.saveErr
	case a.loginErr != "":
		title = "无法更改登录启动"
		detail = "开关已恢复为系统实际状态。" + a.loginErr
	case a.errMsg != "":
		title = "需要留意"
		detail = a.errMsg
	}
	if title == "" {
		return
	}
	t := c.Theme()
	ui.Column(c.Key("settings-notice")).FillWidth().Shrink(0).MinWidth(0).
		Padding(t.Space(3), t.Space(4)).Gap(t.Space(2)).Background(t.Danger.Alpha(.06)).Role(ui.RoleStatus).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			components.Text(c, title).Bold().TextColor(t.Danger).Grow(1).MinWidth(0).Wrap()
			if a.saveErr != "" && !a.readOnly {
				if components.Button(c, "重试保存", components.ButtonSecondary).Clicked() && !a.saving {
					a.save()
				}
				if a.quitBlocked {
					if components.Button(c, "放弃并退出", components.ButtonGhost).Clicked() && !a.saving {
						a.quitting = true
						mygo.App.Quit()
					}
				}
			}
		})
		components.Text(c, detail).FontSize(t.Rem(.875)).TextColor(t.TextMuted).FillWidth().Wrap()
	})
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border)
}

func (a *App) generalCategory() components.SettingsCategory {
	appearance := components.SettingsItem{
		ID: "appearance", Title: "外观", Description: "浅色、深色，或随系统自动切换。", Build: func(c *ui.Context) {
			choices := []struct{ key, title string }{{"system", "跟随系统"}, {"light", "浅色"}, {"dark", "深色"}}
			value := a.cfg.Appearance
			title := choices[0].title
			for _, ch := range choices {
				if ch.key == value {
					title = ch.title
				}
			}
			sel := components.Select(c, &value)
			sel.Trigger.Label("外观").MinWidth(140).Children(func() { components.Text(c, title).SingleLine() })
			sel.Popup(func(ui.Element) {
				for _, ch := range choices {
					sel.Item(ch.key).Label(ch.title).Children(func() { components.Text(c, ch.title) })
				}
			})
			if value != a.cfg.Appearance {
				a.setAppearance(value)
			}
		},
	}
	loginDescription := "启动后显示菜单栏图标，随时使用快捷键。"
	if !a.packaged {
		loginDescription = "仅在打包为 .app 后可用。"
	}
	if a.preview {
		loginDescription = "预览模式不更改系统登录项。"
	}
	login := components.SettingsItem{
		ID: "login", Title: "登录时打开", Description: loginDescription, Build: func(c *ui.Context) {
			v := a.loginEnabled
			components.Switch(c, &v, "登录时打开", components.SwitchOptions{HideLabel: true}).Disabled(!a.packaged || a.preview)
			if v != a.loginEnabled {
				a.setOpenAtLogin(v)
			}
		},
	}
	return components.SettingsCategory{
		ID: "general", Title: "通用", Icon: lucide.Settings(), Description: "管理外观和启动方式。关闭设置窗口后，" + Name + " 仍会在菜单栏运行。",
		Groups: []components.SettingsGroup{
			{ID: "appearance", Title: "外观", Items: []components.SettingsItem{appearance}},
			{ID: "startup", Title: "启动与后台", Items: []components.SettingsItem{
				login,
				{ID: "close", Title: "关闭设置窗口", Description: "关闭或按 ⌘W 只隐藏窗口；从菜单栏可再次打开。退出应用请使用菜单中的「退出 " + Name + "」。"},
			}},
			a.updatesGroup(),
			{ID: "about", Title: "关于", Items: []components.SettingsItem{
				{ID: "name", Title: Name, Description: "零碎小工具集合。"},
				{ID: "version", Title: "版本", Description: applicationVersion(a.version)},
			}},
		},
	}
}

func (a *App) toolCategory(t tool.Tool) components.SettingsCategory {
	var shortcuts []components.SettingsItem
	for _, act := range t.Actions() {
		item := components.SettingsItem{
			ID: act.ID, Title: act.Title, Description: act.Description, Keywords: []string{"快捷键", "shortcut"},
			Build: func(c *ui.Context) {
				v, changed := a.recorder.Field(c, act.ID, a.shortcut(act.ID), act.DefaultShortcut, hotkey.Texts{
					Placeholder: "未设置", Recording: "请按下快捷键…", Clear: "清除快捷键",
					Reset: "恢复默认", Label: act.Title + "的快捷键",
				})
				if changed {
					a.setShortcut(act.ID, v)
				}
			},
		}
		if a.recorder.Active() == act.ID {
			item.Description = "请按下组合键；Esc 取消，Delete 清除。录制期间暂停全局快捷键。"
		}
		if msg, ok := a.shortcutErrs[act.ID]; ok {
			desc := item.Description
			item.DescriptionContent = func(c *ui.Context) {
				components.Text(c, desc+" ")
				components.Text(c, msg).TextColor(c.Theme().Danger)
			}
		}
		shortcuts = append(shortcuts, item)
	}
	shortcuts = append(shortcuts, components.SettingsItem{
		ID: "help", Title: "录制说明", Description: "点击快捷键框后按下组合键。Esc 取消，Delete 清除；组合键须含 ⌃、⌥ 或 ⌘，F1–F12 可单独使用。",
	})
	groups := []components.SettingsGroup{{ID: "shortcuts", Title: "快捷键", Items: shortcuts}}
	if items := t.SettingsItems(a.save); len(items) > 0 {
		groups = append(groups, components.SettingsGroup{ID: "options", Title: "选项", Items: items})
	}
	return components.SettingsCategory{
		ID: t.ID(), Title: t.Title(), Description: t.Description(), Icon: t.Icon(), Section: "工具",
		Groups: groups,
	}
}
