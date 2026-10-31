package app

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"github.com/knothhe/odds/source/internal/uicopy"
)

// Unknown future preferences keep their stored value and display the English
// fallback until the user explicitly chooses a supported language.
func languageOverride(preference string) string {
	switch preference {
	case "", "system":
		return ""
	case "zh-Hans":
		return "zh-Hans"
	default:
		return "en"
	}
}

func (a *App) applyLanguage() { mygo.App.SetLocale(languageOverride(a.cfg.Language)) }

func (a *App) setLanguage(value string) {
	a.cfg.Language = value
	a.applyLanguage()
	a.save()
}

func (a *App) settingsTitle() string {
	title := fmt.Sprintf(uicopy.SettingsWindow(), Name)
	if a.preview {
		title = fmt.Sprintf(uicopy.PreviewWindow(), title)
	}
	return title
}

func (a *App) refreshActions() {
	a.actions = nil
	for _, tool := range a.tools {
		a.actions = append(a.actions, tool.Actions()...)
	}
}

func (a *App) languageChanged() {
	a.refreshActions()
	a.registerShortcuts()
	a.refreshMenus()
	if a.settings != nil {
		a.settings.SetTitle(a.settingsTitle())
		a.settings.Invalidate()
	}
}

func (a *App) languageItem() components.SettingsItem {
	return components.SettingsItem{
		ID: "language", Title: uicopy.Language(),
		Keywords: []string{uicopy.LanguageSystem(), uicopy.LanguageEnglish(), uicopy.LanguageChinese()},
		Build: func(c *ui.Context) {
			choices := []struct{ key, title string }{
				{"system", uicopy.LanguageSystem()}, {"en", uicopy.LanguageEnglish()}, {"zh-Hans", uicopy.LanguageChinese()},
			}
			value, title := a.cfg.Language, uicopy.LanguageEnglish()
			if value == "" {
				value = "system"
			}
			for _, choice := range choices {
				if choice.key == value {
					title = choice.title
				}
			}
			before := value
			selectBox := components.Select(c, &value)
			selectBox.Trigger.Label(uicopy.Language()).MinWidth(140).Children(func() { components.Text(c, title).SingleLine() })
			selectBox.Popup(func(ui.Element) {
				for _, choice := range choices {
					selectBox.Item(choice.key).Label(choice.title).Children(func() { components.Text(c, choice.title) })
				}
			})
			if value != before {
				a.setLanguage(value)
			}
		},
	}
}
