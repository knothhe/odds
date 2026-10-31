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
