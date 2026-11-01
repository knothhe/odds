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
	"github.com/knothhe/odds/source/internal/uicopy"
)

// Settings are the tool's stored preferences.
type Settings struct {
	Highlight bool   `json:"highlight"`
	Color     string `json:"color"` // a key of colors; empty is the system accent color
}

var colors = []struct {
	key, hex string
	title    func() string
}{
	{"", "", uicopy.ColorSystem},
	{"red", "#FF3B30", uicopy.ColorRed},
	{"orange", "#FF9500", uicopy.ColorOrange},
	{"yellow", "#FFCC00", uicopy.ColorYellow},
	{"green", "#34C759", uicopy.ColorGreen},
	{"pink", "#FF2D55", uicopy.ColorPink},
}

// Tool moves and highlights the pointer.
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
func (t *Tool) Title() string { return uicopy.MouseCenter() }
func (t *Tool) Description() string {
	return ""
}
func (t *Tool) Icon() *ui.SVG { return lucide.LocateFixed() }
func (t *Tool) Settings() any { return &t.settings }

func (t *Tool) Actions() []tool.Action {
	return []tool.Action{
		{
			ID: "mouse-center.current", Title: uicopy.CenterCurrent(),
			DefaultShortcut: "Ctrl+Shift+Cmd+C",
			Run:             t.CenterOnCurrent,
		},
		{
			ID: "mouse-center.next", Title: uicopy.CenterNext(),
			DefaultShortcut: "Ctrl+Shift+Cmd+N",
			Run:             t.CenterOnNext,
		},
		{
			ID: "mouse-center.locate", Title: uicopy.LocatePointer(),
			Run: t.Locate,
		},
	}
}

// CenterOnCurrent moves the pointer to the display the user is working on,
// determined by the foreground window rather than the pointer's position.
func (t *Tool) CenterOnCurrent() {
	if d, ok := currentDisplay(mygo.Screen.Displays(), foregroundWindowBounds()); ok {
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
			return c.title()
		}
	}
	return colors[0].title()
}

func (t *Tool) SettingsItems(changed func()) []components.SettingsItem {
	return []components.SettingsItem{
		{ID: "highlight", Title: uicopy.HighlightOnMove(), Build: func(c *ui.Context) {
			before := t.settings.Highlight
			components.Switch(c, &t.settings.Highlight, uicopy.HighlightOnMove(), components.SwitchOptions{HideLabel: true})
			if t.settings.Highlight != before {
				changed()
			}
		}},
		{ID: "color", Title: uicopy.HighlightColor(), Build: func(c *ui.Context) {
			before := t.settings.Color
			sel := components.Select(c, &t.settings.Color)
			sel.Trigger.Label(uicopy.HighlightColor()).MinWidth(140).Children(func() {
				components.Text(c, colorTitle(t.settings.Color)).SingleLine()
			})
			sel.Popup(func(ui.Element) {
				for _, col := range colors {
					sel.Item(col.key).Label(col.title()).Children(func() { components.Text(c, col.title()) })
				}
			})
			if t.settings.Color != before {
				changed()
			}
		}},
		{ID: "preview", Title: uicopy.Preview(), Content: func(c *ui.Context) {
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if components.Button(c, uicopy.Preview(), components.ButtonSecondary).Clicked() {
					t.Locate()
				}
			})
		}},
	}
}

// Close releases the highlight window.
func (t *Tool) Close() { t.highlight.close() }
