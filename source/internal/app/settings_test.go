package app

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"

	"github.com/knothhe/odds/source/internal/tools/mousecenter"
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
	if err := tt.Click("移动后高亮"); err != nil {
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

func TestRecordingDescriptionAndStatusClearAfterEscape(t *testing.T) {
	a := &App{cfg: defaultConfig()}
	tool := mousecenter.New()
	a.tools = append(a.tools, tool)
	a.actions = tool.Actions()
	a.settingsState.Category = tool.ID()
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 700)
	if err := tt.Click("移到当前屏幕中央的快捷键"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("正在录制快捷键") {
		t.Fatal("recording status was not drawn")
	}
	tt.Key(0, ui.KeyEscape)
	if a.recorder.Active() != "" || tt.HasText("正在录制快捷键") {
		t.Fatal("Escape left stale recording status")
	}
	if !tt.HasText("移到鼠标所在屏幕的中央。") {
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
	for _, want := range []string{"移到当前屏幕中央", "⌃⇧⌘C", "⌃⇧⌘N", "未设置", "冲突", "移动后高亮", "高亮颜色"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestSaveFailureRemainsVisibleWhileSearchingAndCanBeRetried(t *testing.T) {
	a := &App{path: filepath.Join(t.TempDir(), "config.json"), cfg: defaultConfig(), saveErr: "磁盘空间不足。"}
	a.settingsState.Query = "不存在的设置"
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 560)
	if !tt.HasText("设置未保存") || !tt.HasText("没有匹配的设置") {
		t.Fatalf("search hid save failure: %v", tt.Texts())
	}
	if err := tt.Click("重试保存"); err != nil {
		t.Fatal(err)
	}
	if a.saveErr != "" || tt.HasText("设置未保存") {
		t.Fatal("successful retry left a stale error")
	}
	if _, err := loadConfig(a.path); err != nil {
		t.Fatal(err)
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
	}{
		{name: "general", category: "general", want: "启动与后台", width: 760, height: 560},
		{name: "tool", category: "mouse-center", want: "⌃⇧⌘C", width: 760, height: 560},
		{name: "dark", category: "mouse-center", want: "⌃⇧⌘C", width: 760, height: 560, dark: true},
		{name: "narrow", category: "mouse-center", want: "设置分类", width: 480, height: 420},
		{name: "failed", category: "mouse-center", want: "重试保存", width: 480, height: 420, saveErr: "磁盘空间不足。"},
		{name: "read-only", category: "general", want: "修改仅本次有效", width: 480, height: 420, readOnly: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a := &App{cfg: defaultConfig(), readOnly: scenario.readOnly, saveErr: scenario.saveErr}
			tool := mousecenter.New()
			a.tools = append(a.tools, tool)
			a.actions = tool.Actions()
			a.settingsState.Category = scenario.category
			tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, scenario.width, scenario.height)
			tt.SetLocale("zh-Hans")
			tt.SetDark(scenario.dark)
			if !tt.HasText(scenario.want) {
				t.Fatalf("missing %q: %v", scenario.want, tt.Texts())
			}
			if scenario.readOnly && tt.HasText("重试保存") {
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
