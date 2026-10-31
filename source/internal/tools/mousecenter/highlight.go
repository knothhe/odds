package mousecenter

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/knothhe/odds/source/internal/uicopy"
	"github.com/oneo-me/onevra/source/theme"
)

const (
	overlaySize      = 280
	highlightPlay    = 700 * time.Millisecond
	highlightStartR  = 120
	highlightEndR    = 22
	highlightStrokeW = 4
)

// highlight shows where the pointer landed: a ring that closes in on it and
// fades, in a click-through panel floating above every app and Space.
type highlight struct {
	win     *mygo.Window
	color   func(c *ui.Context) ui.Color
	gen     int
	pending bool // the next frame starts the animation
	start   time.Time
}

func (h *highlight) show(x, y float64) {
	if h.win == nil {
		h.win = mygo.NewWindow(mygo.WindowOptions{
			Title: uicopy.HighlightWindow, Width: overlaySize, Height: overlaySize,
			Hidden: true, Frameless: true, Transparent: true, DisableShadow: true,
			DisableResize: true, DisableMove: true, AlwaysOnTop: true, SkipTaskbar: true,
			Panel: true, BackgroundColor: "#00000000",
			Content: ui.View(h.view),
		})
		h.win.SetIgnoreMouseEvents(true)
	}
	h.gen++
	gen := h.gen
	h.pending = true
	h.win.SetBounds(mygo.Rectangle{
		X: int(math.Round(x)) - overlaySize/2, Y: int(math.Round(y)) - overlaySize/2,
		Width: overlaySize, Height: overlaySize,
	})
	h.win.ShowInactive()
	h.win.Invalidate()
	time.AfterFunc(highlightPlay+100*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if h.gen == gen && h.win != nil {
				h.win.Hide()
			}
		})
	})
}

func (h *highlight) close() {
	if h.win != nil {
		h.win.Close()
		h.win = nil
	}
}

func (h *highlight) view(c *ui.Context) {
	theme.Apply(c)
	c.Root().Background(ui.Transparent)
	if h.pending {
		h.pending, h.start = false, c.Now()
	}
	p := float64(c.Now().Sub(h.start)) / float64(highlightPlay)
	if p < 1 {
		c.AnimationFrame()
	}
	p = min(max(p, 0), 1)

	ink := c.Theme().Accent
	if h.color != nil {
		ink = h.color(c)
	}
	reduce := c.Preferences().ReduceMotion
	ui.Box(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		radius := float32(highlightEndR + (highlightStartR-highlightEndR)*(1-easeOutCubic(p)))
		if reduce {
			radius = highlightEndR * 1.5
		}
		alpha := float32(1)
		switch {
		case p < 0.12 && !reduce:
			alpha = float32(p / 0.12)
		case p > 0.7:
			alpha = float32((1 - p) / 0.3)
		}
		if alpha <= 0 {
			return
		}
		ring := (&ui.Path{}).Circle(cx, cy, radius)
		// A dark halo keeps the ring visible on light and dark backgrounds.
		pt.StrokePath(ring, highlightStrokeW+4, ui.RGBA(0, 0, 0, 0.28*alpha))
		pt.StrokePath(ring, highlightStrokeW, ink.Alpha(alpha))
		dot := (&ui.Path{}).Circle(cx, cy, 4)
		pt.FillPath(dot, ink.Alpha(alpha))
	})
}

func easeOutCubic(t float64) float64 { return 1 - math.Pow(1-t, 3) }
