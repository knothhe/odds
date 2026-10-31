package app

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"

	"github.com/knothhe/odds/source/internal/hotkey"
	"github.com/knothhe/odds/source/internal/tool"
	"github.com/knothhe/odds/source/internal/uicopy"
)

func (a *App) openSettings() {
	if a.quitting {
		return
	}
	if a.settings == nil {
		title := fmt.Sprintf(uicopy.SettingsWindow, Name)
		if a.preview {
			title = fmt.Sprintf(uicopy.PreviewWindow, title)
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
	if a.settings != nil {
		a.settingsShortcuts(c, a.settings.Close)
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
			Narrow: w < 620, Title: uicopy.Settings, SearchLabel: uicopy.SearchSettings, SearchPlaceholder: uicopy.SearchSettings,
			CategoryLabel: uicopy.SettingsCategories, ResultsTitle: uicopy.SearchResults, EmptyText: uicopy.NoMatchingSettings,
			HeaderActions: func(c *ui.Context) {
				if status := a.settingsStatus(); status != "" {
					components.Text(c, status).FontSize(c.Theme().Rem(.8)).TextColor(c.Theme().TextMuted).Role(ui.RoleStatus)
				}
			},
		})
	})
}

func (a *App) settingsShortcuts(c *ui.Context, close func()) {
	// The plain shell nests Content under its own layout. OnShortcut would
	// scope the handler to that subtree and miss a newly opened, unfocused
	// window. Query the window shortcut; the recorder still owns all keys
	// while active.
	if a.recorder.Active() == "" && c.Shortcut(ui.Cmd, ui.KeyW) {
		close()
	}
}

func (a *App) settingsStatus() string {
	switch {
	case a.readOnly:
		return uicopy.SessionOnly
	case a.saving:
		return uicopy.Saving
	case a.saveErr != "":
		return uicopy.Unsaved
	case a.recorder.Active() != "":
		return uicopy.RecordingShortcut
	case a.preview:
		return uicopy.PreviewSettings
	default:
		return ""
	}
}

// Notices live outside the searchable catalog so navigating or filtering never
// hides a persistence failure. Saving keeps the current input and runtime state.
func (a *App) settingsNotice(c *ui.Context) {
	title, detail := "", ""
	switch {
	case a.readOnly:
		title = uicopy.SettingsUnavailable
		detail = uicopy.ReadOnlyDetail
		if a.readOnlyDetail != "" {
			title, detail = uicopy.FutureConfigTitle, a.readOnlyDetail
		}
	case a.saveErr != "":
		title = uicopy.SettingsNotSaved
		detail = fmt.Sprintf(uicopy.SaveFailureDetail, a.saveErr)
	case a.loginErr != "":
		title = uicopy.LoginFailure
		detail = fmt.Sprintf(uicopy.LoginFailureDetail, a.loginErr)
	case a.errMsg != "":
		title = uicopy.Attention
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
				if components.Button(c, uicopy.RetrySave, components.ButtonSecondary).Clicked() && !a.saving {
					a.save()
				}
				if a.quitBlocked {
					if components.Button(c, uicopy.DiscardAndQuit, components.ButtonGhost).Clicked() && !a.saving {
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
		ID: "appearance", Title: uicopy.Appearance,
		Keywords: []string{uicopy.AppearanceSystem, uicopy.AppearanceLight, uicopy.AppearanceDark},
		Build: func(c *ui.Context) {
			choices := []struct{ key, title string }{{"system", uicopy.AppearanceSystem}, {"light", uicopy.AppearanceLight}, {"dark", uicopy.AppearanceDark}}
			value := a.cfg.Appearance
			title := choices[0].title
			for _, ch := range choices {
				if ch.key == value {
					title = ch.title
				}
			}
			sel := components.Select(c, &value)
			sel.Trigger.Label(uicopy.Appearance).MinWidth(140).Children(func() { components.Text(c, title).SingleLine() })
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
	loginDescription := ""
	if !a.packaged {
		loginDescription = uicopy.LoginNeedsBundle
	}
	if a.preview {
		loginDescription = uicopy.LoginPreview
	}
	login := components.SettingsItem{
		ID: "login", Title: uicopy.OpenAtLogin, Description: loginDescription, Build: func(c *ui.Context) {
			v := a.loginEnabled
			components.Switch(c, &v, uicopy.OpenAtLogin, components.SwitchOptions{HideLabel: true}).Disabled(!a.packaged || a.preview)
			if v != a.loginEnabled {
				a.setOpenAtLogin(v)
			}
		},
	}
	return components.SettingsCategory{
		ID: "general", Title: uicopy.General, Icon: lucide.Settings(),
		Groups: []components.SettingsGroup{
			{ID: "preferences", Items: []components.SettingsItem{appearance, login}},
			a.updatesGroup(),
			a.aboutGroup(),
		},
	}
}

func (a *App) toolCategory(t tool.Tool) components.SettingsCategory {
	var shortcuts []components.SettingsItem
	for _, act := range t.Actions() {
		item := components.SettingsItem{
			ID: act.ID, Title: act.Title, Description: act.Description, Keywords: []string{uicopy.Shortcuts, uicopy.ShortcutSearchTerm},
			Build: func(c *ui.Context) {
				v, changed := a.recorder.Field(c, act.ID, a.shortcut(act.ID), act.DefaultShortcut, hotkey.Texts{
					Placeholder: uicopy.ShortcutUnset, Recording: uicopy.ShortcutPrompt, Clear: uicopy.ShortcutClear,
					Reset: uicopy.ShortcutReset, Label: fmt.Sprintf(uicopy.ShortcutLabel, act.Title),
				})
				if changed {
					a.setShortcut(act.ID, v)
				}
			},
		}
		if a.recorder.Active() == act.ID {
			item.Description = uicopy.ShortcutHelp
		}
		if msg, ok := a.shortcutErrs[act.ID]; ok {
			desc := item.Description
			item.DescriptionContent = func(c *ui.Context) {
				if desc != "" {
					components.Text(c, desc+" ")
				}
				components.Text(c, msg).TextColor(c.Theme().Danger)
			}
		}
		shortcuts = append(shortcuts, item)
	}

	groups := []components.SettingsGroup{{ID: "shortcuts", Title: uicopy.Shortcuts, Items: shortcuts}}
	if items := t.SettingsItems(a.save); len(items) > 0 {
		groups = append(groups, components.SettingsGroup{ID: "options", Title: uicopy.Options, Items: items})
	}
	return components.SettingsCategory{
		ID: t.ID(), Title: t.Title(), Description: t.Description(), Icon: t.Icon(), Section: uicopy.Tools,
		Groups: groups,
	}
}
