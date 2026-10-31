// Package hotkey converts between key presses, accelerators such as
// "Ctrl+Alt+Cmd+C" (the format of mygo.GlobalShortcut) and the symbols macOS
// shows for them, and provides a control that records a shortcut.
package hotkey

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

var keyNames = map[ui.Key]string{
	ui.KeyEnter: "Enter", ui.KeyEscape: "Escape", ui.KeyBackspace: "Backspace", ui.KeyTab: "Tab",
	ui.KeySpace: "Space", ui.KeyDelete: "Delete", ui.KeyInsert: "Insert", ui.KeyHome: "Home",
	ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown", ui.KeyLeft: "Left",
	ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/",
	ui.KeySemicolon: ";", ui.KeyQuote: "'", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]",
	ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
}

func keyName(k ui.Key) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + k - ui.KeyA))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + k - ui.Key0))
	case k >= ui.KeyF1 && k <= ui.KeyF12:
		return "F" + itoa(int(k-ui.KeyF1)+1)
	}
	return keyNames[k]
}

func isFunctionKey(k ui.Key) bool { return k >= ui.KeyF1 && k <= ui.KeyF12 }

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// FromKey returns the accelerator for key pressed with mods, and false when
// it cannot be a global shortcut: a key without a name, or a key other than
// a function key pressed without Control, Option or Command.
func FromKey(mods ui.Modifiers, key ui.Key) (string, bool) {
	name := keyName(key)
	if name == "" {
		return "", false
	}
	if mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 && !isFunctionKey(key) {
		return "", false
	}
	var parts []string
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}, {ui.Super, "Cmd"}} {
		if mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, name), "+"), true
}

var symbols = map[string]string{
	"enter": "↩", "return": "↩", "escape": "⎋", "esc": "⎋", "backspace": "⌫", "tab": "⇥",
	"space": "Space", "delete": "⌦", "del": "⌦", "home": "↖", "end": "↘", "pageup": "⇞",
	"pagedown": "⇟", "left": "←", "right": "→", "up": "↑", "down": "↓",
}

// Format shows an accelerator as macOS does, e.g. "⌃⌥⌘C", with the
// modifiers in the system's order whatever order acc names them in.
func Format(acc string) string {
	if acc == "" {
		return ""
	}
	parts := strings.Split(acc, "+")
	if strings.HasSuffix(acc, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}
	var ctrl, alt, shift, cmd bool
	key := ""
	for i, p := range parts {
		if i < len(parts)-1 {
			switch strings.ToLower(strings.TrimSpace(p)) {
			case "ctrl", "control":
				ctrl = true
			case "alt", "option", "altgr":
				alt = true
			case "shift":
				shift = true
			case "cmd", "command", "super", "meta", "cmdorctrl", "commandorcontrol":
				cmd = true
			}
			continue
		}
		key = strings.TrimSpace(p)
	}
	var b strings.Builder
	for _, m := range []struct {
		on  bool
		sym string
	}{{ctrl, "⌃"}, {alt, "⌥"}, {shift, "⇧"}, {cmd, "⌘"}} {
		if m.on {
			b.WriteString(m.sym)
		}
	}
	if s, ok := symbols[strings.ToLower(key)]; ok {
		key = s
	} else {
		key = strings.ToUpper(key)
	}
	b.WriteString(key)
	return b.String()
}
