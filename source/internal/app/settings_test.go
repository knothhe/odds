package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"

	"github.com/knothhe/odds/source/internal/tools/mousecenter"
	"github.com/knothhe/odds/source/internal/uicopy"
)

func TestHighlightSwitchPersistsItsChangedValue(t *testing.T) {
	a := &App{path: filepath.Join(t.TempDir(), "config.json"), cfg: defaultConfig()}
	tool := mousecenter.New()
	a.tools = append(a.tools, tool)
	a.actions = tool.Actions()
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		tool.SettingsItems(a.save)[0].Build(c)
	}, 300, 100)
	if err := tt.Click(uicopy.HighlightOnMove()); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(a.path)
	if err != nil {
		t.Fatal(err)
	}
	var settings mousecenter.Settings
	if err := json.Unmarshal(loaded.Tools[tool.ID()], &settings); err != nil {
		t.Fatalf("changed switch was not persisted: %v", err)
	}
	if settings.Highlight {
		t.Fatal("switch saved its previous value")
	}
}

func TestCloseShortcutWorksBeforeAnyControlHasFocus(t *testing.T) {
	a := &App{}
	closed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() { a.settingsShortcuts(c, func() { closed++ }) })
	}, 300, 100)
	tt.Key(ui.Cmd, ui.KeyW)
	if closed != 1 {
		t.Fatal("Cmd+W missed the nested shell without focus")
	}
}

func TestRecordingDescriptionAndStatusClearAfterEscape(t *testing.T) {
	a := &App{cfg: defaultConfig()}
	tool := mousecenter.New()
	a.tools = append(a.tools, tool)
	a.actions = tool.Actions()
	a.settingsState.Category = tool.ID()
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 700)
	if err := tt.Click(fmt.Sprintf(uicopy.ShortcutLabel(), uicopy.CenterCurrent())); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(uicopy.RecordingShortcut()) || !tt.HasText(uicopy.ShortcutHelp()) {
		t.Fatal("recording status or contextual instructions were not drawn")
	}
	tt.Key(0, ui.KeyEscape)
	if a.recorder.Active() != "" || tt.HasText(uicopy.RecordingShortcut()) {
		t.Fatal("Escape left stale recording status")
	}
	if tt.HasText(uicopy.ShortcutHelp()) {
		t.Fatal("Escape left stale recording instructions")
	}
}

// The tool page builds its shortcut rows and options without a window.
func TestToolCategory(t *testing.T) {
	a := &App{cfg: defaultConfig(), shortcutErrs: map[string]string{"mouse-center.next": "冲突"}}
	tool := mousecenter.New()
	a.tools = append(a.tools, tool)
	a.actions = tool.Actions()
	var state components.SettingsState
	view := func(c *ui.Context) {
		theme.Apply(c)
		components.Settings(c, &state, []components.SettingsCategory{a.toolCategory(tool)}, components.SettingsOptions{HideSidebar: true, HideSearch: true})
	}
	tt := ui.NewTester(view, 760, 900)
	for _, want := range []string{uicopy.CenterCurrent(), "⌃⇧⌘C", "⌃⇧⌘N", uicopy.ShortcutUnset(), "冲突", uicopy.HighlightOnMove(), uicopy.HighlightColor()} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestSaveFailureRemainsVisibleWhileSearchingAndCanBeRetried(t *testing.T) {
	a := &App{path: filepath.Join(t.TempDir(), "config.json"), cfg: defaultConfig(), saveErr: uicopy.SaveDiskFull()}
	a.settingsState.Query = "不存在的设置"
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 560)
	if !tt.HasText(uicopy.SettingsNotSaved()) || !tt.HasText(uicopy.NoMatchingSettings()) {
		t.Fatalf("search hid save failure: %v", tt.Texts())
	}
	if err := tt.Click(uicopy.RetrySave()); err != nil {
		t.Fatal(err)
	}
	if a.saveErr != "" || tt.HasText(uicopy.SettingsNotSaved()) {
		t.Fatal("successful retry left a stale error")
	}
	if _, err := loadConfig(a.path); err != nil {
		t.Fatal(err)
	}
}

func TestAppearanceChoicesRemainSearchable(t *testing.T) {
	for _, query := range []string{uicopy.AppearanceSystem(), uicopy.AppearanceLight(), uicopy.AppearanceDark()} {
		t.Run(query, func(t *testing.T) {
			a := &App{cfg: defaultConfig()}
			a.settingsState.Query = query
			tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 560)
			if !tt.HasText(uicopy.Appearance()) || tt.HasText(uicopy.NoMatchingSettings()) {
				t.Fatalf("appearance choice %q did not find its setting: %v", query, tt.Texts())
			}
		})
	}
}

// These exercise the actual native UI builders, including scrollable content
// at the minimum window size. Optional screenshots are local review artifacts.
func TestSettingsStatesAndLayouts(t *testing.T) {
	for _, scenario := range []struct {
		name, category, want string
		width, height        int
		dark, readOnly       bool
		saveErr              string
		readOnlyDetail       string
	}{
		{name: "general", category: "general", want: uicopy.OpenAtLogin(), width: 760, height: 560},
		{name: "tool", category: "mouse-center", want: "⌃⇧⌘C", width: 760, height: 560},
		{name: "dark", category: "mouse-center", want: "⌃⇧⌘C", width: 760, height: 560, dark: true},
		{name: "narrow", category: "mouse-center", want: uicopy.SettingsCategories(), width: 480, height: 420},
		{name: "failed", category: "mouse-center", want: uicopy.RetrySave(), width: 480, height: 420, saveErr: uicopy.SaveDiskFull()},
		{name: "read-only", category: "general", want: uicopy.SessionOnly(), width: 480, height: 420, readOnly: true},
		{name: "future-config", category: "general", want: uicopy.FutureConfigTitle(), width: 480, height: 420, readOnly: true, readOnlyDetail: fmt.Sprintf(uicopy.FutureConfigDetail(), 2, configSchema)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a := &App{cfg: defaultConfig(), readOnly: scenario.readOnly, saveErr: scenario.saveErr, readOnlyDetail: scenario.readOnlyDetail}
			tool := mousecenter.New()
			a.tools = append(a.tools, tool)
			a.actions = tool.Actions()
			a.settingsState.Category = scenario.category
			tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, scenario.width, scenario.height)
			tt.SetLocale("en")
			tt.SetDark(scenario.dark)
			if !tt.HasText(scenario.want) {
				t.Fatalf("missing %q: %v", scenario.want, tt.Texts())
			}
			if scenario.readOnly && tt.HasText(uicopy.RetrySave()) {
				t.Fatal("unreadable original must not offer an overwrite")
			}
			if directory := os.Getenv("ODDS_UI_SCREENSHOTS"); directory != "" {
				f, err := os.Create(filepath.Join(directory, scenario.name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("screenshot: %v, close: %v", err, closeErr)
				}
			}
		})
	}
}

func TestShortcutDefaultsAndOverrides(t *testing.T) {
	a := &App{cfg: defaultConfig(), actions: mousecenter.New().Actions()}
	if got := a.shortcut("mouse-center.current"); got != "Ctrl+Shift+Cmd+C" {
		t.Errorf("default = %q", got)
	}
	a.cfg.Shortcuts["mouse-center.current"] = ""
	if got := a.shortcut("mouse-center.current"); got != "" {
		t.Errorf("disabled = %q", got)
	}
}
