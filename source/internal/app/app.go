// Package app is the odds menu bar application: it hosts the tools, binds
// their actions to global shortcuts, and owns the tray and Settings.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/oneo-me/onevra/source/components"

	"github.com/knothhe/odds/source/internal/hotkey"
	"github.com/knothhe/odds/source/internal/tool"
	"github.com/knothhe/odds/source/internal/tools/mousecenter"
)

// Name is the application's name.
const Name = "odds"

// Version is shown in Settings; keep it equal to version in mygo.json.
const Version = "0.1.0"

// App holds all application state. Everything runs on the main thread.
type App struct {
	path         string
	cfg          Config
	tools        []tool.Tool
	actions      []tool.Action
	shortcutErrs map[string]string
	suspended    bool
	quitting     bool
	loginEnabled bool
	loginErr     string
	errMsg       string
	readOnly     bool // the config file could not be read: never save over it
	backedUp     bool // an invalid config file was moved aside

	tray          *mygo.Tray
	settings      *mygo.Window
	settingsState components.SettingsState
	recorder      hotkey.Recorder
}

// registry lists the tools in the order Settings and the tray show them.
// Add new tools here.
func registry() []tool.Tool {
	return []tool.Tool{
		mousecenter.New(),
	}
}

// Run starts odds and returns when it quits.
func Run() error {
	mygo.App.SetName(Name)
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(base, Name)
	mygo.App.SetPath(mygo.PathUserData, dir)

	a := &App{path: filepath.Join(dir, "config.json"), tools: registry()}
	a.recorder.OnActiveChange = a.suspendShortcuts
	a.cfg, err = loadConfig(a.path)
	var corrupt *corruptError
	switch {
	case errors.As(err, &corrupt):
		a.recoverConfig(err)
	case err != nil:
		// Unreadable: keep the file as it is and save nothing over it.
		a.readOnly = true
		a.report("读取设置失败，本次运行的修改不会保存", err)
	}
	for _, t := range a.tools {
		a.actions = append(a.actions, t.Actions()...)
		if s := t.Settings(); s != nil {
			if raw, ok := a.cfg.Tools[t.ID()]; ok {
				if err := json.Unmarshal(raw, s); err != nil {
					a.recoverConfig(fmt.Errorf("「%s」的设置：%w", t.Title(), err))
				}
			}
		}
	}

	mygo.Theme.SetSource(mygo.ThemeSource(a.cfg.Appearance))
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {}) // a menu bar app keeps running
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		mygo.GlobalShortcut.UnregisterAll()
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) { a.openSettings() })
	mygo.App.WhenReady(a.start)
	return mygo.App.Run()
}

func (a *App) start() {
	a.registerShortcuts()
	if err := a.createTray(); err != nil {
		a.report("无法创建菜单栏图标", err)
	}
	a.refreshMenus()
	if !a.cfg.Welcomed || a.tray == nil {
		a.cfg.Welcomed = true
		a.save()
		a.openSettings()
	}
}

// recoverConfig handles a configuration file that is not valid: it moves
// the file aside so the defaults can be saved without losing it, or, when
// it cannot, keeps it and saves nothing for this run.
func (a *App) recoverConfig(cause error) {
	if a.readOnly || a.backedUp {
		return
	}
	backup, err := backupConfig(a.path, time.Now())
	if err != nil {
		a.readOnly = true
		a.report("设置文件无效且无法备份，本次运行的修改不会保存", fmt.Errorf("%v；备份：%w", cause, err))
		return
	}
	a.backedUp = true
	a.report("设置文件无效，已备份到 "+backup+" 并改用默认设置", cause)
}

func (a *App) report(msg string, err error) {
	a.errMsg = msg + "：" + err.Error()
	log.Printf("%s: %v", msg, err)
}

// save writes the configuration, including every tool's settings.
func (a *App) save() {
	for _, t := range a.tools {
		if s := t.Settings(); s != nil {
			raw, err := json.Marshal(s)
			if err != nil {
				a.report("保存设置失败", err)
				return
			}
			a.cfg.Tools[t.ID()] = raw
		}
	}
	if a.readOnly {
		return
	}
	if err := saveConfig(a.path, a.cfg); err != nil {
		a.report("保存设置失败", err)
	}
}

func (a *App) setAppearance(v string) {
	a.cfg.Appearance = v
	mygo.Theme.SetSource(mygo.ThemeSource(v))
	a.save()
}

func (a *App) setOpenAtLogin(v bool) {
	if err := mygo.App.SetOpenAtLogin(v); err != nil {
		a.loginErr = err.Error()
		a.loginEnabled = mygo.App.OpenAtLogin()
		return
	}
	a.loginErr = ""
	a.loginEnabled = v
	a.refreshMenus()
}
