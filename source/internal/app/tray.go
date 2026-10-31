package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"

	"github.com/knothhe/odds/source/internal/uicopy"
)

func (a *App) createTray() error {
	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID: "main", Icon: trayIcon(), IconIsTemplate: true, ToolTip: mygo.App.Name(),
	})
	return err
}

// refreshMenus rebuilds the tray menu and the app menu, which show the
// actions with their current shortcuts.
func (a *App) refreshMenus() {
	settings := &mygo.MenuItem{ID: "settings", Label: uicopy.SettingsMenu(), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }}
	quit := &mygo.MenuItem{ID: "quit", Label: fmt.Sprintf(uicopy.QuitApp(), Name), Accelerator: "Cmd+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }}
	if a.suspended {
		// While a shortcut is recorded the menus claim no keys, so ⌘Q, ⌘C
		// and the like reach the recorder.
		settings.Accelerator, quit.Accelerator = "", ""
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{settings, a.updateMenu(), mygo.Separator(), quit}},
		}))
	} else {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{settings, a.updateMenu(), mygo.Separator(), quit}},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))
	}
	if a.tray == nil {
		return
	}
	var items []*mygo.MenuItem
	for i, t := range a.tools {
		if i > 0 {
			items = append(items, mygo.Separator())
		}
		items = append(items, &mygo.MenuItem{Label: t.Title(), Disabled: true})
		for _, act := range t.Actions() {
			run := act.Run
			items = append(items, &mygo.MenuItem{
				ID: act.ID, Label: act.Title, Accelerator: a.shortcut(act.ID),
				Click: func(*mygo.MenuItem, *mygo.Window) { run() },
			})
		}
	}
	items = append(items, mygo.Separator(),
		&mygo.MenuItem{ID: "settings", Label: uicopy.SettingsMenu(), Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
		a.updateMenu(),
		mygo.Separator(),
		&mygo.MenuItem{ID: "quit", Label: fmt.Sprintf(uicopy.QuitApp(), Name), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	)
	a.tray.SetMenu(mygo.NewMenu(items))
}

func (a *App) updateMenu() *mygo.MenuItem {
	if !a.updateReady {
		return &mygo.MenuItem{Label: uicopy.CheckUpdates(), Disabled: true}
	}
	return updater.MenuItem()
}

// trayIcon draws a crosshair as a 36 × 36 template image (18 points at 2×).
func trayIcon() []byte {
	const size, ss = 36, 4 // pixels, supersamples per axis
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	inside := func(x, y float64) bool {
		d := math.Hypot(x-c, y-c)
		switch {
		case math.Abs(d-10) <= 1.6: // ring
			return true
		case d <= 2.6: // center dot
			return true
		}
		// Ticks crossing the ring.
		ax, ay := math.Abs(x-c), math.Abs(y-c)
		tick := func(along, across float64) bool { return across <= 1.3 && along >= 6 && along <= 16 }
		return tick(ax, ay) || tick(ay, ax)
	}
	for py := range size {
		for px := range size {
			n := 0
			for sy := range ss {
				for sx := range ss {
					if inside(float64(px)+(float64(sx)+0.5)/ss, float64(py)+(float64(sy)+0.5)/ss) {
						n++
					}
				}
			}
			img.SetNRGBA(px, py, color.NRGBA{A: uint8(255 * n / (ss * ss))})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}
