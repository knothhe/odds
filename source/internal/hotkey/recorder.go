package hotkey

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// Recorder records global shortcuts. One Recorder serves every shortcut
// field of a window: at most one of them records at a time.
//
// Call BeginFrame before building the fields of a frame and EndFrame after:
// recording stops when its field is no longer built, as when the page
// showing it changes.
type Recorder struct {
	active string
	seen   bool
	// OnActiveChange is called when recording starts or stops, at once, so
	// the app can suspend its global shortcuts and the window's key
	// equivalents while a shortcut is recorded.
	OnActiveChange func(recording bool)
}

// Active returns the ID of the field recording now, or empty.
func (r *Recorder) Active() string { return r.active }

func (r *Recorder) start(id string) {
	was := r.active != ""
	r.active, r.seen = id, true
	if !was && r.OnActiveChange != nil {
		r.OnActiveChange(true)
	}
}

// Stop ends recording, if a field is recording.
func (r *Recorder) Stop() {
	if r.active == "" {
		return
	}
	r.active = ""
	if r.OnActiveChange != nil {
		r.OnActiveChange(false)
	}
}

// BeginFrame starts a frame of fields.
func (r *Recorder) BeginFrame() { r.seen = false }

// EndFrame stops recording when the recording field was not built.
func (r *Recorder) EndFrame() {
	if r.active != "" && !r.seen {
		r.Stop()
	}
}

// Texts are the field's user-facing strings, owned by the app.
type Texts struct {
	Placeholder, Recording, Clear, Reset, Label string
}

type fieldState struct {
	armed   bool // the field has had the focus since recording started
	pending *string
}

// Field builds a shortcut field for the accelerator acc, with a button that
// clears it and, when def differs from acc, one that restores def. Click
// the field, then press the shortcut; Escape cancels, Delete or Backspace
// clears. It returns the new accelerator and whether it changed.
func (r *Recorder) Field(c *ui.Context, id, acc, def string, texts Texts) (string, bool) {
	t := c.Theme()
	result, changed := acc, false

	ui.Row(c.Key("hotkey:" + id)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		box := ui.Row(c.Key("field:"+id)).Focusable().Role(ui.RoleButton).Label(texts.Label).
			Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Center).
			MinWidth(132).Height(28).Padding(0, t.Space(3)).Radius(t.Radius)
		st := ui.Local(box, "state", func() fieldState { return fieldState{} })

		if st.pending != nil {
			result, changed = *st.pending, *st.pending != acc
			st.pending = nil
			if r.active == id {
				r.Stop()
			}
		}
		if box.Clicked() {
			if r.active == id {
				r.Stop()
			} else {
				r.start(id)
				st.armed = false
			}
		}
		if r.active == id {
			// Recording stops when the focus leaves the field.
			if box.Focused() {
				st.armed = true
			} else if st.armed {
				r.Stop()
			} else {
				box.Focus()
			}
		}
		recording := r.active == id
		if recording {
			r.seen = true
		}

		if recording {
			box.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
			box.HandleInput(func(ev ui.InputEvent) bool {
				switch ev.Kind {
				case ui.InputKeyUp:
					return true
				case ui.InputKeyDown:
					key := ev.PhysicalKey
					if key == ui.KeyUnknown {
						key = ev.Key
					}
					switch {
					case ev.Mods == 0 && key == ui.KeyEscape:
						v := acc
						st.pending = &v
					case ev.Mods == 0 && (key == ui.KeyBackspace || key == ui.KeyDelete):
						v := ""
						st.pending = &v
					default:
						if v, ok := FromKey(ev.Mods, key); ok {
							st.pending = &v
						}
					}
					return true
				case ui.InputText, ui.InputCompose, ui.InputCommand:
					return true
				}
				return false
			})
		} else {
			box.Background(t.Surface).Border(1, t.Border)
		}

		box.Children(func() {
			// Modifier keys held on their own come as no key event; the
			// context reports them and builds a frame as they change.
			mods := c.Modifiers() & (ui.Ctrl | ui.Alt | ui.Shift | ui.Super)
			switch {
			case recording && mods != 0:
				components.Text(c, modifierSymbols(mods)+"…").TextColor(t.Accent)
			case recording:
				components.Text(c, texts.Recording).TextColor(t.Accent)
			case result == "":
				components.Text(c, texts.Placeholder).TextColor(t.TextMuted)
			default:
				components.Text(c, Format(result))
			}
		})

		if result != "" && !recording {
			if components.IconButton(c.Key("clear:"+id), lucide.X(), texts.Clear, components.IconButtonCompact).Clicked() {
				result, changed = "", true
			}
		}
		if result != def && def != "" && !recording {
			if components.IconButton(c.Key("reset:"+id), lucide.RotateCcw(), texts.Reset, components.IconButtonCompact).Clicked() {
				result, changed = def, true
			}
		}
	})
	return result, changed
}

func modifierSymbols(m ui.Modifiers) string {
	s := ""
	if m&ui.Ctrl != 0 {
		s += "⌃"
	}
	if m&ui.Alt != 0 {
		s += "⌥"
	}
	if m&ui.Shift != 0 {
		s += "⇧"
	}
	if m&ui.Super != 0 {
		s += "⌘"
	}
	return s
}
