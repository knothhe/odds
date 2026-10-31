package app

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"github.com/knothhe/odds/source/internal/buildinfo"
	"github.com/knothhe/odds/source/internal/thirdparty"
	"github.com/knothhe/odds/source/internal/uicopy"
)

const projectURL = "https://github.com/knothhe/odds"

var linkedVersions = func() map[string]string {
	versions := map[string]string{"Go": runtime.Version()}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, module := range info.Deps {
			version := module.Version
			if module.Replace != nil {
				if module.Replace.Version != "" {
					version = module.Replace.Version
				} else {
					version = fmt.Sprintf(uicopy.ModifiedDependency, version)
				}
			}
			versions[module.Path] = version
		}
	}
	return versions
}()

func (a *App) versionText() string {
	parts := []string{Name, applicationVersion(a.version), buildinfo.Channel}
	if buildinfo.Commit != "unknown" {
		parts = append(parts, buildinfo.Commit)
	}
	return strings.Join(parts, " · ")
}

func (a *App) aboutGroup() components.SettingsGroup {
	return components.SettingsGroup{ID: "about", Title: uicopy.About, Items: []components.SettingsItem{
		{ID: "name", Title: Name, Description: applicationVersion(a.version), Keywords: []string{uicopy.Version}, Build: func(c *ui.Context) {
			components.Button(c, uicopy.CopyVersion, components.ButtonSecondary).OnClick(func() {
				c.Services().WriteClipboard(a.versionText())
				c.Announce(uicopy.VersionCopied)
			})
		}},
		{ID: "project", Keywords: []string{uicopy.ProjectPage}, Content: func(c *ui.Context) {
			components.Button(c, uicopy.ProjectPage, components.ButtonGhost).OnClick(func() {
				c.OpenURLThen(projectURL, func(err error) {
					if err != nil {
						a.report(uicopy.ProjectOpenFailure, err)
					}
				})
			})
		}},
		{ID: "dependencies", Keywords: []string{uicopy.Dependencies}, Content: func(c *ui.Context) {
			parts := components.Collapsible(c, uicopy.Dependencies, &a.dependenciesOpen, components.CollapsibleOptions{}, func() {
				components.Text(c, uicopy.DependenciesDetail).TextColor(c.Theme().TextMuted).Wrap()
				if a.licenseOpen == nil {
					a.licenseOpen = map[string]*bool{}
				}
				for _, dependency := range thirdparty.Dependencies {
					open := a.licenseOpen[dependency.Path]
					if open == nil {
						open = new(bool)
						a.licenseOpen[dependency.Path] = open
					}
					version := linkedVersions[dependency.Path]
					if version == "" {
						version = dependency.Version
					}
					part := components.Collapsible(c.Key(dependency.Path), dependency.Path+" "+version, open, components.CollapsibleOptions{}, func() {
						for _, text := range dependency.Texts {
							ui.Column(c.Key(text.Name)).FillWidth().MinWidth(0).Gap(c.Theme().Space(2)).Children(func() {
								components.Text(c, text.Name).Bold().Wrap()
								components.Text(c, text.Text).FontSize(c.Theme().Rem(.8)).FillWidth().Wrap()
							})
						}
					})
					part.Root.FillWidth().MinWidth(0)
				}
			})
			parts.Root.FillWidth().MinWidth(0)
		}},
	}}
}
