package app

import (
	"fmt"
	"log"

	"github.com/egoist/mygo"

	"github.com/knothhe/odds/source/internal/uicopy"
)

// shortcut returns the accelerator bound to an action.
func (a *App) shortcut(actionID string) string {
	if v, ok := a.cfg.Shortcuts[actionID]; ok {
		return v
	}
	for _, act := range a.actions {
		if act.ID == actionID {
			return act.DefaultShortcut
		}
	}
	return ""
}

func (a *App) setShortcut(actionID, acc string) {
	a.cfg.Shortcuts[actionID] = acc
	a.save()
	a.registerShortcuts()
	a.refreshMenus()
	if a.settings != nil {
		a.settings.Invalidate()
	}
}

// registerShortcuts binds every action's shortcut, recording in
// shortcutErrs why one could not be bound.
func (a *App) registerShortcuts() {
	mygo.GlobalShortcut.UnregisterAll()
	a.shortcutErrs = map[string]string{}
	if a.suspended || a.preview {
		return
	}
	owner := map[string]string{} // normalized accelerator → action title
	for _, act := range a.actions {
		acc := a.shortcut(act.ID)
		if acc == "" {
			continue
		}
		key := normalize(acc)
		if other, ok := owner[key]; ok {
			a.shortcutErrs[act.ID] = fmt.Sprintf(uicopy.ShortcutConflict, other)
			continue
		}
		run := act.Run
		if err := mygo.GlobalShortcut.Register(acc, run); err != nil {
			log.Printf("register %s (%s): %v", act.ID, acc, err)
			a.shortcutErrs[act.ID] = uicopy.ShortcutOccupied
			continue
		}
		owner[key] = act.Title
	}
}

// suspendShortcuts turns the global shortcuts and the menus' key
// equivalents off while a shortcut is being recorded, so pressing one
// records it instead of running its action. The recorder calls it as
// recording starts and stops.
func (a *App) suspendShortcuts(suspend bool) {
	if a.suspended == suspend {
		return
	}
	a.suspended = suspend
	if suspend {
		mygo.GlobalShortcut.UnregisterAll()
	} else {
		a.registerShortcuts()
	}
	a.refreshMenus()
	// The recorder may stop while its row is being built, after the header
	// and help text have already read its old state. Native rendering needs
	// a fresh frame to clear that earlier content.
	if a.settings != nil {
		a.settings.Invalidate()
	}
}
