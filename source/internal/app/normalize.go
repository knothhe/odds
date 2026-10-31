package app

import (
	"slices"
	"strings"
)

// normalize gives equal accelerators one spelling, whatever the order and
// names of their modifiers, so that conflicts can be found.
func normalize(acc string) string {
	parts := strings.Split(acc, "+")
	if strings.HasSuffix(acc, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}
	key := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	var mods []string
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl", "control":
			mods = append(mods, "ctrl")
		case "alt", "option", "altgr":
			mods = append(mods, "alt")
		case "shift":
			mods = append(mods, "shift")
		case "cmd", "command", "super", "meta", "cmdorctrl", "commandorcontrol":
			mods = append(mods, "cmd")
		}
	}
	slices.Sort(mods)
	mods = slices.Compact(mods)
	return strings.Join(append(mods, key), "+")
}
