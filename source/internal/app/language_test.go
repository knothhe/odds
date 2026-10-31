package app

import (
	"encoding/json"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/knothhe/odds/source/internal/tools/mousecenter"
	"github.com/knothhe/odds/source/internal/uicopy"
	"github.com/oneo-me/onevra/source/theme"
)

func TestLanguagePreferenceMigrationAndPersistence(t *testing.T) {
	for _, value := range []string{"system", "en", "zh-Hans", "future-language"} {
		var cfg Config
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"language":%q,"appearance":"dark"}`, value)), &cfg); err != nil {
			t.Fatal(err)
		}
		cfg.normalize()
		snapshot, err := json.Marshal(cfg.snapshot())
		if err != nil {
			t.Fatal(err)
		}
		var loaded Config
		if err = json.Unmarshal(snapshot, &loaded); err != nil || loaded.Language != value {
			t.Fatalf("language lost: %s %v", snapshot, err)
		}
	}
	var old Config
	if err := json.Unmarshal([]byte(`{"appearance":"dark"}`), &old); err != nil || old.Language != "system" {
		t.Fatalf("old settings do not follow system: %+v %v", old, err)
	}
	for preference, want := range map[string]string{"system": "", "": "", "en": "en", "zh-Hans": "zh-Hans", "unknown": "en"} {
		if got := languageOverride(preference); got != want {
			t.Errorf("%s: %s, want %s", preference, got, want)
		}
	}
}

func TestBilingualSettingsKeepNavigationAndDrafts(t *testing.T) {
	locale := "en"
	i18n.SetLocaleFunc(func() string { return locale })
	defer i18n.SetLocaleFunc(nil)
	a := &App{cfg: defaultConfig(), preview: true}
	tool := mousecenter.New()
	a.tools = append(a.tools, tool)
	a.actions = tool.Actions()
	a.cfg.Shortcuts["mouse-center.current"] = "Ctrl+Alt+K"
	a.settingsState.Category = tool.ID()
	for _, width := range []int{760, 480} {
		tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, width, 560)
		for _, lang := range []string{"en", "zh-Hans", "en"} {
			locale = lang
			tt.SetLocale(lang)
			tt.Frame()
			if !tt.HasText(uicopy.CenterCurrent()) {
				t.Fatalf("%s missing translated tool: %v", lang, tt.Texts())
			}
			if a.settingsState.Category != tool.ID() || a.shortcut("mouse-center.current") != "Ctrl+Alt+K" {
				t.Fatal("language changed navigation or draft")
			}
			// General remains usable at both widths; scrollable content contains the
			// translated language selector without changing the setting's stored value.
			a.settingsState.Category = "general"
			tt.Frame()
			if !tt.HasText(uicopy.Language()) {
				t.Fatalf("language setting missing: %v", tt.Texts())
			}
			a.settingsState.Category = tool.ID()
			tt.Advance(time.Second)
		}
	}
}

func TestFailureDetailsReformatInTheNewLanguage(t *testing.T) {
	locale := "en"
	i18n.SetLocaleFunc(func() string { return locale })
	defer i18n.SetLocaleFunc(nil)
	a := &App{cfg: defaultConfig(), saveCause: syscall.ENOSPC}
	if got := a.saveMessage(); got != "The disk is full." {
		t.Fatal(got)
	}
	locale = "zh-Hans"
	if got := a.saveMessage(); got != "磁盘空间不足。" {
		t.Fatal(got)
	}
	a.readOnly, a.readOnlySchema = true, 2
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsNotice(c) }, 760, 220)
	if !tt.HasText(uicopy.FutureConfigTitle()) {
		t.Fatal(tt.Texts())
	}
	locale = "en"
	tt.Frame()
	if !tt.HasText(uicopy.FutureConfigTitle()) {
		t.Fatal(tt.Texts())
	}
}
