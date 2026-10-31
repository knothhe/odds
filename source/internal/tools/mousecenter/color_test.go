package mousecenter

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// "系统强调色" follows the accent color of System Settings, not the theme's.
func TestInkColorFollowsSystemAccent(t *testing.T) {
	tool := New()
	var got ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		got = tool.inkColor(c)
	}, 100, 100)
	system := ui.Hex("#34C7AA")
	tt.SetPreferences(ui.Preferences{Accent: system})
	tt.Frame()
	if got != system {
		t.Fatalf("ink %v, want system accent %v", got, system)
	}
	tool.settings.Color = "red"
	tt.Frame()
	if got != ui.Hex("#FF3B30") {
		t.Fatalf("ink %v, want red", got)
	}
}
