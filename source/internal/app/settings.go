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
		shell := plain.NewShell(plain.ShellOptions{
			Title: "odds 设置", Width: 760, Height: 560, MinWidth: 480, MinHeight: 420,
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
	a.loginEnabled = mygo.App.OpenAtLogin()
	if a.settings.IsMinimized() {
		a.settings.Restore()
	}
	a.settings.Show()
	mygo.App.Focus()
	a.settings.Focus()
}

func (a *App) settingsView(c *ui.Context) {
	if a.recorder.Active() == "" {
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
	components.Settings(c, &a.settingsState, categories, components.SettingsOptions{
		Narrow: w < 620, Title: "设置", SearchPlaceholder: "搜索设置", EmptyText: "没有匹配的设置",
	})
}

func (a *App) generalCategory() components.SettingsCategory {
	items := []components.SettingsItem{
		{ID: "appearance", Title: "外观", Build: func(c *ui.Context) {
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
		}},
		{ID: "login", Title: "登录时打开", Description: "仅在打包为 .app 后生效。", Build: func(c *ui.Context) {
			v := a.loginEnabled
			if components.Switch(c, &v, "登录时打开", components.SwitchOptions{HideLabel: true}).Changed() {
				a.setOpenAtLogin(v)
			}
		}},
		{ID: "about", Title: "关于", Description: "odds " + Version + " · 零碎小工具集合"},
	}
	if a.loginErr != "" || a.errMsg != "" {
		items = append(items, components.SettingsItem{ID: "error", Title: "错误", DescriptionContent: func(c *ui.Context) {
			msg := a.errMsg
			if a.loginErr != "" {
				msg = "登录时打开：" + a.loginErr
			}
			components.Text(c, msg).TextColor(c.Theme().Danger)
		}})
	}
	return components.SettingsCategory{
		ID: "general", Title: "通用", Icon: lucide.Settings(),
		Groups: []components.SettingsGroup{{ID: "general", Items: items}},
	}
}

func (a *App) toolCategory(t tool.Tool) components.SettingsCategory {
	var shortcuts []components.SettingsItem
	for _, act := range t.Actions() {
		item := components.SettingsItem{
			ID: act.ID, Title: act.Title, Description: act.Description, Keywords: []string{"快捷键", "shortcut"},
			Build: func(c *ui.Context) {
				v, changed := a.recorder.Field(c, act.ID, a.shortcut(act.ID), act.DefaultShortcut, hotkey.Texts{
					Placeholder: "录制快捷键", Recording: "请按下快捷键…", Clear: "清除快捷键",
					Reset: "恢复默认", Label: act.Title + "的快捷键",
				})
				if changed {
					a.setShortcut(act.ID, v)
				}
			},
		}
		if msg, ok := a.shortcutErrs[act.ID]; ok {
			desc := act.Description
			item.DescriptionContent = func(c *ui.Context) {
				components.Text(c, desc+" ")
				components.Text(c, msg).TextColor(c.Theme().Danger)
			}
		}
		shortcuts = append(shortcuts, item)
	}
	groups := []components.SettingsGroup{{ID: "shortcuts", Title: "快捷键", Items: shortcuts}}
	if items := t.SettingsItems(a.save); len(items) > 0 {
		groups = append(groups, components.SettingsGroup{ID: "options", Title: "选项", Items: items})
	}
	return components.SettingsCategory{
		ID: t.ID(), Title: t.Title(), Description: t.Description(), Icon: t.Icon(), Section: "工具",
		Groups: groups,
	}
}
