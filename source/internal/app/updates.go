package app

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"github.com/knothhe/odds/source/internal/buildinfo"
	"github.com/knothhe/odds/source/internal/uicopy"
	"github.com/knothhe/odds/source/internal/updateclient"
)

func applicationBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(exe)))
}

func matchesInstallation(target string) bool {
	actual := applicationBundle()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil || resolved != actual {
		return false
	}
	record, err := buildinfo.Read(actual)
	return err == nil && record.Schema == 1 && record.Channel == "dev" &&
		record.BundleID == "com.knothhe.odds.dev" && record.Name == "odds Dev" && buildinfo.Channel == "dev"
}

func (a *App) setupUpdates() {
	record, err := buildinfo.Read(applicationBundle())
	switch {
	case a.preview:
		a.updateReason = uicopy.UpdatesPreview
	case buildinfo.Channel != "release":
		a.updateReason = uicopy.UpdatesDev
	case err != nil || !buildinfo.ReleaseEligible(record, mygo.App.Version(), a.preview):
		a.updateReason = uicopy.UpdatesUnconfigured
	case !mygo.Updater.Enabled():
		a.updateReason = uicopy.UpdatesUnwritable
	default:
		mygo.Updater.SetSource(updateclient.New(buildinfo.Repository, buildinfo.PublicKey, mygo.App.Version(), nil))
		a.updateReady = true
		a.updateReason = uicopy.UpdatesAutomatic
		mygo.Use(native.New(updater.Options{Language: uicopy.Locale}))
	}
}

func (a *App) updatesGroup() components.SettingsGroup {
	return components.SettingsGroup{ID: "updates", Title: uicopy.SoftwareUpdates, Items: []components.SettingsItem{{
		ID: "check", Description: a.updateReason, Keywords: []string{uicopy.CheckUpdates},
		Build: func(c *ui.Context) {
			components.Button(c, uicopy.CheckUpdates, components.ButtonSecondary).Disabled(!a.updateReady).OnClick(func() {
				if a.updateReady {
					updater.CheckForUpdates()
				}
			})
		},
	}}}
}
