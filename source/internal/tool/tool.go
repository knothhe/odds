// Package tool defines the contract every odds tool implements. The app
// registers tools, binds their actions to global shortcuts, lists them in the
// tray menu and gives each one a page in Settings.
package tool

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// Action is something a tool does on demand, from a global shortcut or the
// tray menu. Run is called on the main thread.
type Action struct {
	// ID is stable across releases; the app stores the user's shortcut under
	// it. Prefix it with the tool's ID, e.g. "mouse-center.current".
	ID          string
	Title       string
	Description string
	// DefaultShortcut is an accelerator such as "Ctrl+Shift+Cmd+C", or empty
	// for an action that has no shortcut until the user records one.
	DefaultShortcut string
	Run             func()
}

// Tool is one small utility.
type Tool interface {
	// ID is stable across releases and names the tool's stored settings.
	ID() string
	Title() string
	Description() string
	Icon() *ui.SVG
	Actions() []Action
	// Settings returns a pointer to the tool's JSON-serializable settings,
	// which the app loads before Start and saves after changed is called, or
	// nil for a tool without settings.
	Settings() any
	// SettingsItems builds the tool's own rows in Settings. Call changed
	// after editing the value Settings returns.
	SettingsItems(changed func()) []components.SettingsItem
}
