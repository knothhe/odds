// Package mousecenter moves the mouse pointer to the center of a display
// and highlights where it landed.
package mousecenter

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"github.com/knothhe/odds/source/internal/cursor"
	"github.com/knothhe/odds/source/internal/tool"
)

// Settings are the tool's stored preferences.
type Settings struct {
	Highlight bool   `json:"highlight"`
	Color     string `json:"color"` // a key of colors; empty is the system accent color
}

var colors = []struct{ key, title, hex string }{
	{"", "系统强调色", ""},
	{"red", "红色", "#FF3B30"},
	{"orange", "橙色", "#FF9500"},
	{"yellow", "黄色", "#FFCC00"},
	{"green", "绿色", "#34C759"},
	{"pink", "粉色", "#FF2D55"},
}

// Tool is the "鼠标居中" tool.
type Tool struct {
	settings  Settings
	highlight highlight
}

var _ tool.Tool = (*Tool)(nil)

// New returns the tool with its default settings.
func New() *Tool {
	t := &Tool{settings: Settings{Highlight: true}}
	t.highlight.color = t.inkColor
	return t
}

func (t *Tool) ID() string    { return "mouse-center" }
func (t *Tool) Title() string { return "鼠标居中" }
func (t *Tool) Description() string {
	return "把鼠标指针移到屏幕中央，并高亮显示它的位置。"
}
func (t *Tool) Icon() *ui.SVG { return lucide.LocateFixed() }
func (t *Tool) Settings() any { return &t.settings }

func (t *Tool) Actions() []tool.Action {
	return []tool.Action{
		{
			ID: "mouse-center.current", Title: "移到当前屏幕中央",
			Description:     "移到鼠标所在屏幕的中央。",
			DefaultShortcut: "Ctrl+Shift+Cmd+C",
			Run:             t.CenterOnCurrent,
		},
		{
			ID: "mouse-center.next", Title: "移到下一块屏幕中央",
			Description:     "多显示器时，依次移到下一块屏幕的中央。",
			DefaultShortcut: "Ctrl+Shift+Cmd+N",
			Run:             t.CenterOnNext,
		},
		{
			ID: "mouse-center.locate", Title: "高亮鼠标位置",
			Description: "不移动鼠标，只显示它在哪里。",
			Run:         t.Locate,
		},
	}
}

// CenterOnCurrent moves the pointer to the center of the display it is on.
func (t *Tool) CenterOnCurrent() {
	if d, ok := displayAt(mygo.Screen.Displays(), mygo.Screen.CursorScreenPoint()); ok {
		t.moveTo(center(d.Bounds))
	}
}

// CenterOnNext moves the pointer to the center of the next display.
func (t *Tool) CenterOnNext() {
	displays := mygo.Screen.Displays()
	cur, ok := displayAt(displays, mygo.Screen.CursorScreenPoint())
	if !ok {
		return
	}
	if d, ok := nextDisplay(displays, cur); ok {
		t.moveTo(center(d.Bounds))
	}
}

// Locate highlights the pointer where it is.
func (t *Tool) Locate() {
	p := mygo.Screen.CursorScreenPoint()
	t.highlight.show(float64(p.X), float64(p.Y))
}

func (t *Tool) moveTo(x, y float64) {
	if err := cursor.MoveTo(x, y); err != nil {
		log.Printf("mouse-center: %v", err)
		return
	}
	if t.settings.Highlight {
		t.highlight.show(x, y)
	}
}

// inkColor is the highlight's color: the chosen one, else the accent color
// set in System Settings. Onevra's theme keeps an accent of its own, so the
// system's comes from the desktop preferences, falling back to the theme's
// where the platform reports none.
func (t *Tool) inkColor(c *ui.Context) ui.Color {
	for _, col := range colors {
		if col.key == t.settings.Color && col.hex != "" {
			return ui.Hex(col.hex)
		}
	}
	if accent := c.Preferences().Accent; accent.A > 0 {
		return accent
	}
	return c.Theme().Accent
}

func colorTitle(key string) string {
	for _, c := range colors {
		if c.key == key {
			return c.title
		}
	}
	return colors[0].title
}

func (t *Tool) SettingsItems(changed func()) []components.SettingsItem {
	return []components.SettingsItem{
		{ID: "highlight", Title: "移动后高亮", Description: "移动后在指针处显示一个收缩的圆环。", Build: func(c *ui.Context) {
			if components.Switch(c, &t.settings.Highlight, "移动后高亮", components.SwitchOptions{HideLabel: true}).Changed() {
				changed()
			}
		}},
		{ID: "color", Title: "高亮颜色", Build: func(c *ui.Context) {
			before := t.settings.Color
			sel := components.Select(c, &t.settings.Color)
			sel.Trigger.Label("高亮颜色").MinWidth(140).Children(func() {
				components.Text(c, colorTitle(t.settings.Color)).SingleLine()
			})
			sel.Popup(func(ui.Element) {
				for _, col := range colors {
					sel.Item(col.key).Label(col.title).Children(func() { components.Text(c, col.title) })
				}
			})
			if t.settings.Color != before {
				changed()
			}
		}},
		{ID: "preview", Title: "预览", Description: "在鼠标当前位置播放一次高亮效果。", Build: func(c *ui.Context) {
			if components.Button(c, "预览", components.ButtonSecondary).Clicked() {
				t.Locate()
			}
		}},
	}
}

// Close releases the highlight window.
func (t *Tool) Close() { t.highlight.close() }
