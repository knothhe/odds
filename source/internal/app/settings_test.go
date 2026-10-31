package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"

	"github.com/knothhe/odds/source/internal/tools/mousecenter"
)

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
	for _, want := range []string{"移到当前屏幕中央", "⌃⇧⌘C", "⌃⇧⌘N", "录制快捷键", "冲突", "移动后高亮", "高亮颜色"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
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
