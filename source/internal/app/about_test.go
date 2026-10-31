package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/knothhe/odds/source/internal/thirdparty"
	"github.com/knothhe/odds/source/internal/uicopy"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"
)

func TestAboutCopiesVersionAndShowsOfflineLicenses(t *testing.T) {
	a := &App{cfg: defaultConfig(), version: "1.2.3-rc.1"}
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		components.Settings(c, &a.settingsState, []components.SettingsCategory{{ID: "about", Groups: []components.SettingsGroup{a.aboutGroup()}}}, components.SettingsOptions{HideSidebar: true, HideSearch: true})
	}, 760, 900)
	if err := tt.Click(uicopy.CopyVersion()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tt.Clipboard(), "1.2.3-rc.1") {
		t.Fatal("copied version was guessed from another source")
	}
	if err := tt.Click(uicopy.ProjectPage()); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != projectURL {
		t.Fatalf("wrong project link: %v", urls)
	}
	if err := tt.Click(uicopy.Dependencies()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("github.com/egoist/mygo") || !tt.HasText("v0.7.3") {
		t.Fatalf("missing dependencies: %v", tt.Texts())
	}
	for _, label := range tt.Texts() {
		if strings.HasPrefix(label, "Go go") {
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Advance(300 * time.Millisecond)
			if !tt.HasText("Copyright") {
				t.Fatal("offline license text is unavailable")
			}
			return
		}
	}
	t.Fatal("missing Go build information")
}

func TestExpandedLicenseFitsSettingsWidth(t *testing.T) {
	for _, width := range []int{480, 760} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			open := true
			a := &App{cfg: defaultConfig(), dependenciesOpen: true, licenseOpen: map[string]*bool{"Go": &open}}
			tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, width, 560)
			tt.Advance(time.Second)
			license := thirdparty.Dependencies[0].Texts[0].Text
			r, ok := tt.Find(license)
			if !ok || r.X < 0 || r.X+r.W > float32(width) {
				t.Fatalf("license overflows settings: %+v", r)
			}
			a.settingsState.Query = uicopy.ProjectPage()
			tt.Frame()
			if !tt.HasText(uicopy.ProjectPage()) || tt.HasText(uicopy.NoMatchingSettings()) {
				t.Fatal("project entry is not searchable")
			}
		})
	}
}
