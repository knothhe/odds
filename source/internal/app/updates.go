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
		a.updateReason = "预览模式不检查正式版更新。"
	case buildinfo.Channel != "release":
		a.updateReason = "Dev 版本不参与正式版更新。"
	case err != nil || !buildinfo.ReleaseEligible(record, mygo.App.Version(), a.preview):
		a.updateReason = "此构建未配置正式版自动更新。"
	case !mygo.Updater.Enabled():
		a.updateReason = "当前位置无法安装更新，请移到可写的应用程序目录。"
	default:
		mygo.Updater.SetSource(updateclient.New(buildinfo.Repository, buildinfo.PublicKey, mygo.App.Version(), nil))
		a.updateReady = true
		a.updateReason = "每 24 小时自动检查，发现新版本后可查看更新说明并选择安装。"
		mygo.Use(native.New(updater.Options{Language: "zh-Hans"}))
	}
}

func (a *App) updatesGroup() components.SettingsGroup {
	return components.SettingsGroup{ID: "updates", Title: "软件更新", Items: []components.SettingsItem{{
		ID: "check", Title: "检查更新", Description: a.updateReason,
		Build: func(c *ui.Context) {
			components.Button(c, "检查更新…", components.ButtonSecondary).Disabled(!a.updateReady).OnClick(func() {
				if a.updateReady {
					updater.CheckForUpdates()
				}
			})
		},
	}}}
}
