package mousecenter

import (
	"testing"

	"github.com/egoist/mygo"
)

var (
	left    = mygo.Display{ID: 2, Bounds: mygo.Rectangle{X: -1920, Y: 0, Width: 1920, Height: 1080}}
	primary = mygo.Display{ID: 1, Primary: true, Bounds: mygo.Rectangle{X: 0, Y: 0, Width: 1512, Height: 982}}
	above   = mygo.Display{ID: 3, Bounds: mygo.Rectangle{X: 0, Y: -1440, Width: 2560, Height: 1440}}
	all     = []mygo.Display{primary, left, above}
)

func TestCenter(t *testing.T) {
	if x, y := center(left.Bounds); x != -960 || y != 540 {
		t.Fatalf("center = %v, %v", x, y)
	}
	if x, y := center(primary.Bounds); x != 756 || y != 491 {
		t.Fatalf("center = %v, %v", x, y)
	}
}

func TestCurrentDisplayFollowsForegroundWindow(t *testing.T) {
	// Put the primary display last so an unavailable window cannot accidentally
	// pass by choosing the first display. No cursor position is needed to select.
	displays := []mygo.Display{left, above, primary}
	for _, tt := range []struct {
		name   string
		window mygo.Rectangle
		want   int64
	}{
		{"primary", mygo.Rectangle{X: 100, Y: 100, Width: 1000, Height: 700}, 1},
		{"left", mygo.Rectangle{X: -1700, Y: 100, Width: 1000, Height: 700}, 2},
		{"above", mygo.Rectangle{X: 200, Y: -1200, Width: 1200, Height: 800}, 3},
		{"spanning mostly primary", mygo.Rectangle{X: -200, Y: 100, Width: 1200, Height: 700}, 1},
		{"spanning mostly left", mygo.Rectangle{X: -1000, Y: 100, Width: 1200, Height: 700}, 2},
		{"no foreground window", mygo.Rectangle{}, 1},
		{"invalid window", mygo.Rectangle{Width: -10, Height: 600}, 1},
		{"disconnected window display", mygo.Rectangle{X: 5000, Y: 5000, Width: 800, Height: 600}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := currentDisplay(displays, tt.window)
			if !ok || got.ID != tt.want {
				t.Fatalf("currentDisplay(%+v) = %d, %t; want %d", tt.window, got.ID, ok, tt.want)
			}
		})
	}
	if _, ok := currentDisplay(nil, mygo.Rectangle{Width: 800, Height: 600}); ok {
		t.Error("reported a current display without any displays")
	}
	if got, ok := currentDisplay([]mygo.Display{left, above}, mygo.Rectangle{}); !ok || got.ID != left.ID {
		t.Error("did not fall back to an available display without a primary")
	}
}

func TestCurrentDisplayUsesOverlapRatherThanWindowCenter(t *testing.T) {
	displays := []mygo.Display{
		{ID: 1, Primary: true, Bounds: mygo.Rectangle{Width: 1000, Height: 1000}},
		{ID: 2, Bounds: mygo.Rectangle{X: 1000, Y: 350, Width: 1000, Height: 300}},
	}
	// The center lies on display 2, but most of the window is on display 1.
	window := mygo.Rectangle{X: 600, Width: 1000, Height: 1000}
	if got, ok := currentDisplay(displays, window); !ok || got.ID != 1 {
		t.Fatalf("currentDisplay = %d, %t; want display 1", got.ID, ok)
	}
}

func TestDisplayAt(t *testing.T) {
	for _, tt := range []struct {
		p    mygo.Point
		want int64
	}{
		{mygo.Point{X: 10, Y: 10}, 1},
		{mygo.Point{X: -5, Y: 500}, 2},
		{mygo.Point{X: 2000, Y: -10}, 3},
		{mygo.Point{X: 1600, Y: 500}, 1}, // off every display: nearest
	} {
		if d, _ := displayAt(all, tt.p); d.ID != tt.want {
			t.Errorf("displayAt(%v) = %d, want %d", tt.p, d.ID, tt.want)
		}
	}
	if _, ok := displayAt(nil, mygo.Point{}); ok {
		t.Error("displayAt(nil) reported a display")
	}
}

func TestNextDisplay(t *testing.T) {
	// Left to right, then top to bottom: left (-1920,0), above (0,-1440), primary (0,0).
	for cur, want := range map[int64]int64{2: 3, 3: 1, 1: 2} {
		var d mygo.Display
		for _, x := range all {
			if x.ID == cur {
				d = x
			}
		}
		if got, _ := nextDisplay(all, d); got.ID != want {
			t.Errorf("nextDisplay(%d) = %d, want %d", cur, got.ID, want)
		}
	}
	if got, _ := nextDisplay([]mygo.Display{primary}, primary); got.ID != 1 {
		t.Errorf("single display: got %d", got.ID)
	}
}
