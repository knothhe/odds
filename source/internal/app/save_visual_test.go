package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/knothhe/odds/source/internal/tools/mousecenter"
	"github.com/knothhe/odds/source/internal/uicopy"
	"github.com/oneo-me/onevra/source/theme"
)

// Compare the entire native settings surface through a real controlled writer,
// with normal animations, a hovered/focused retry button and editable fields.
func TestSaveRetryFramesKeepControlsStable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, duration := range []time.Duration{0, 40 * time.Millisecond, 600 * time.Millisecond} {
			t.Run(fmt.Sprintf("dark-%t/%s", dark, duration), func(t *testing.T) {
				a := &App{cfg: defaultConfig(), saveErr: uicopy.SaveDiskFull}
				tool := mousecenter.New()
				a.tools = append(a.tools, tool)
				a.actions = tool.Actions()
				a.settingsState.Category = tool.ID()
				started := make(chan struct{}, 4)
				release := make(chan error, 4)
				completed := make(chan saveResult, 4)
				var writes atomic.Int32
				a.writer = newConfigWriter(func(Config) error { writes.Add(1); started <- struct{}{}; return <-release }, func(r uint64, e error) { completed <- saveResult{r, e} })
				defer a.writer.close()
				// Always release the worker if an assertion fails.
				defer close(release)
				tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); a.settingsView(c) }, 760, 560)
				tt.SetLocale(uicopy.Locale)
				tt.SetDark(dark)
				retry, ok := tt.Find(uicopy.RetrySave)
				if !ok {
					t.Fatal("missing retry")
				}
				tt.Move(retry.X+retry.W/2, retry.Y+retry.H/2)
				for i := 0; i < 20 && !tt.Focused(uicopy.RetrySave); i++ {
					tt.Key(0, ui.KeyTab)
				}
				if !tt.Focused(uicopy.RetrySave) {
					t.Fatal("retry is not keyboard reachable")
				}
				tt.Advance(time.Second)
				before := tt.Image()
				oldStatus, _ := tt.Find(uicopy.Unsaved)
				a.save()
				receive(t, started)
				tt.Frame()
				newStatus, _ := tt.Find(uicopy.Saving)
				allowed := pixelRect(oldStatus).Union(pixelRect(newStatus)).Inset(-2)
				check := func() {
					t.Helper()
					if current, _ := tt.Find(uicopy.RetrySave); current != retry {
						t.Fatal("retry moved during saving")
					}
					if !tt.Focused(uicopy.RetrySave) {
						t.Fatal("saving stole retry focus")
					}
					assertPixelsOutside(t, before, tt.Image(), allowed)
				}
				check()
				for elapsed := time.Duration(0); elapsed < duration; elapsed += 16 * time.Millisecond {
					tt.Advance(16 * time.Millisecond)
					check()
				}
				// Mouse and keyboard paths must both refuse a duplicate write.
				tt.Click(uicopy.RetrySave)
				tt.Key(0, ui.KeyEnter)
				if a.saveRevision != 1 || writes.Load() != 1 {
					t.Fatal("retry submitted overlapping saves")
				}
				release <- errors.New(uicopy.SaveDiskFull)
				result := receive(t, completed)
				a.saved(result.revision, result.err)
				tt.Frame()
				tt.Advance(time.Second)
				if !tt.HasText(uicopy.SaveDiskFull) {
					t.Fatal("retry hid the failure")
				}
				assertPixelsOutside(t, before, tt.Image(), allowed)
				// Success may remove the notice; no stale animation may reinsert it.
				tt.Key(0, ui.KeyEnter)
				receive(t, started)
				a.cfg.Appearance = "dark"
				a.save() // a later edit belongs to the next snapshot
				release <- nil
				receive(t, started)
				receive(t, completed)
				release <- nil
				result = receive(t, completed)
				a.saved(result.revision, result.err)
				tt.Frame()
				if a.cfg.Appearance != "dark" || tt.HasText(uicopy.SettingsNotSaved) {
					t.Fatal("success lost later input or kept stale notice")
				}
				tt.Advance(time.Second)
				stable := tt.Image()
				tt.Advance(time.Second)
				tt.Frame()
				if !bytes.Equal(stable.Pix, tt.Image().Pix) {
					t.Fatal("completion left a late visual transition")
				}
			})
		}
	}
}

func pixelRect(r ui.Rect) image.Rectangle {
	return image.Rect(int(r.X), int(r.Y), int(r.X+r.W+1), int(r.Y+r.H+1))
}

func assertPixelsOutside(t *testing.T, before, after *image.RGBA, allowed image.Rectangle) {
	t.Helper()
	if before.Bounds() != after.Bounds() {
		t.Fatal("surface dimensions changed")
	}
	for y := before.Bounds().Min.Y; y < before.Bounds().Max.Y; y++ {
		for x := before.Bounds().Min.X; x < before.Bounds().Max.X; x++ {
			if !image.Pt(x, y).In(allowed) && before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				t.Fatalf("unexpected visual change at (%d,%d)", x, y)
			}
		}
	}
}
