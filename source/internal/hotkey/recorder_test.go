package hotkey

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

type harness struct {
	rec     Recorder
	acc     string
	changes int
	hidden  bool // the field is not built, as on another settings page
	closed  int  // ⌘W presses the window took
	events  []bool
}

func newHarness(acc string) *harness {
	h := &harness{acc: acc}
	h.rec.OnActiveChange = func(on bool) { h.events = append(h.events, on) }
	return h
}

func (h *harness) view(c *ui.Context) {
	if h.rec.Active() == "" {
		c.OnShortcut(ui.Cmd, ui.KeyW, func() { h.closed++ })
	}
	h.rec.BeginFrame()
	defer h.rec.EndFrame()
	if h.hidden {
		return
	}
	v, changed := h.rec.Field(c, "act", h.acc, "Ctrl+Alt+Cmd+C", Texts{
		Placeholder: "Record", Recording: "Press keys", Clear: "Clear", Reset: "Reset", Label: "Shortcut",
	})
	if changed {
		h.acc = v
		h.changes++
	}
}

func TestRecorderRecordsShortcut(t *testing.T) {
	h := newHarness("Ctrl+Alt+Cmd+C")
	tt := ui.NewTester(h.view, 400, 80)
	if !tt.HasText("⌃⌥⌘C") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("Shortcut"); err != nil {
		t.Fatal(err)
	}
	if h.rec.Active() != "act" || !tt.HasText("Press keys") {
		t.Fatalf("not recording: active %q, texts %q", h.rec.Active(), tt.Texts())
	}
	tt.Key(0, ui.KeyA) // no modifier: ignored
	if h.rec.Active() != "act" {
		t.Fatal("a plain letter ended recording")
	}
	tt.Key(ui.Alt|ui.Shift, ui.KeyM)
	if h.acc != "Alt+Shift+M" || h.rec.Active() != "" || h.changes != 1 {
		t.Fatalf("acc %q, active %q, changes %d", h.acc, h.rec.Active(), h.changes)
	}
	if !tt.HasText("⌥⇧M") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("Reset"); err != nil {
		t.Fatal(err)
	}
	if h.acc != "Ctrl+Alt+Cmd+C" {
		t.Fatalf("reset gave %q", h.acc)
	}
}

func TestRecorderEscapeCancelsAndDeleteClears(t *testing.T) {
	h := newHarness("F5")
	tt := ui.NewTester(h.view, 400, 80)
	_ = tt.Click("Shortcut")
	tt.Key(0, ui.KeyEscape)
	if h.acc != "F5" || h.rec.Active() != "" || h.changes != 0 {
		t.Fatalf("escape: acc %q, active %q, changes %d", h.acc, h.rec.Active(), h.changes)
	}
	_ = tt.Click("Shortcut")
	tt.Key(0, ui.KeyBackspace)
	if h.acc != "" || !tt.HasText("Record") {
		t.Fatalf("backspace: acc %q, texts %q", h.acc, tt.Texts())
	}
}

// The app suspends and restores its shortcuts through OnActiveChange, which
// must be told as soon as recording ends, without waiting for another frame.
func TestRecorderReportsStartAndStopAtOnce(t *testing.T) {
	h := newHarness("")
	tt := ui.NewTester(h.view, 400, 80)
	_ = tt.Click("Shortcut")
	tt.Key(0, ui.KeyF5)
	if h.acc != "F5" || len(h.events) != 2 || !h.events[0] || h.events[1] {
		t.Fatalf("acc %q, events %v", h.acc, h.events)
	}
	_ = tt.Click("Shortcut")
	tt.Key(0, ui.KeyEscape)
	if len(h.events) != 4 || h.events[3] {
		t.Fatalf("escape: events %v", h.events)
	}
}

// Recording stops when its field is no longer built, as when the user
// switches to another settings page.
func TestRecorderStopsWhenFieldDisappears(t *testing.T) {
	h := newHarness("F5")
	tt := ui.NewTester(h.view, 400, 80)
	_ = tt.Click("Shortcut")
	h.hidden = true
	tt.Frame()
	if h.rec.Active() != "" || len(h.events) != 2 || h.events[1] {
		t.Fatalf("active %q, events %v", h.rec.Active(), h.events)
	}
}

func TestRecorderShowsHeldModifiers(t *testing.T) {
	h := newHarness("F5")
	tt := ui.NewTester(h.view, 400, 80)
	_ = tt.Click("Shortcut")
	tt.HoldModifiers(ui.Ctrl | ui.Alt)
	if !tt.HasText("⌃⌥…") {
		t.Fatalf("texts %q", tt.Texts())
	}
	tt.HoldModifiers(0)
	if !tt.HasText("Press keys") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

// A shortcut the window claims while idle, as ⌘W, is recorded instead.
func TestRecorderTakesWindowShortcut(t *testing.T) {
	h := newHarness("F5")
	tt := ui.NewTester(h.view, 400, 80)
	_ = tt.Click("Shortcut")
	tt.Key(ui.Super, ui.KeyW)
	if h.acc != "Cmd+W" || h.closed != 0 {
		t.Fatalf("acc %q, closed %d", h.acc, h.closed)
	}
	tt.Key(ui.Super, ui.KeyW)
	if h.closed != 1 {
		t.Fatalf("after recording ⌘W should close; closed %d", h.closed)
	}
}
