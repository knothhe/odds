package hotkey

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFromKey(t *testing.T) {
	tests := []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
		ok   bool
	}{
		{ui.Ctrl | ui.Alt | ui.Super, ui.KeyC, "Ctrl+Alt+Cmd+C", true},
		{ui.Super | ui.Shift, ui.Key1, "Shift+Cmd+1", true},
		{ui.Alt, ui.KeySpace, "Alt+Space", true},
		{0, ui.KeyF5, "F5", true},
		{ui.Ctrl, ui.KeySlash, "Ctrl+/", true},
		{0, ui.KeyA, "", false},
		{ui.Shift, ui.KeyA, "", false},
		{ui.Ctrl, ui.KeyUnknown, "", false},
	}
	for _, tt := range tests {
		got, ok := FromKey(tt.mods, tt.key)
		if got != tt.want || ok != tt.ok {
			t.Errorf("FromKey(%v, %v) = %q, %v; want %q, %v", tt.mods, tt.key, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFormat(t *testing.T) {
	for in, want := range map[string]string{
		"Ctrl+Alt+Cmd+C":   "⌃⌥⌘C",
		"Cmd+Shift+Ctrl+1": "⌃⇧⌘1",
		"Option+Space":     "⌥Space",
		"Ctrl+Up":          "⌃↑",
		"F5":               "F5",
		"CmdOrCtrl++":      "⌘+",
		"":                 "",
	} {
		if got := Format(in); got != want {
			t.Errorf("Format(%q) = %q, want %q", in, got, want)
		}
	}
}
