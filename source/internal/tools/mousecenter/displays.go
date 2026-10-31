package mousecenter

import (
	"cmp"
	"slices"

	"github.com/egoist/mygo"
)

// center returns the middle of r in screen coordinates.
func center(r mygo.Rectangle) (x, y float64) {
	return float64(r.X) + float64(r.Width)/2, float64(r.Y) + float64(r.Height)/2
}

func contains(r mygo.Rectangle, p mygo.Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// displayAt returns the display under p, else the nearest one.
func displayAt(displays []mygo.Display, p mygo.Point) (mygo.Display, bool) {
	if len(displays) == 0 {
		return mygo.Display{}, false
	}
	best, bestDist := displays[0], int64(-1)
	for _, d := range displays {
		if contains(d.Bounds, p) {
			return d, true
		}
		cx, cy := center(d.Bounds)
		dx, dy := int64(cx)-int64(p.X), int64(cy)-int64(p.Y)
		if dist := dx*dx + dy*dy; bestDist < 0 || dist < bestDist {
			best, bestDist = d, dist
		}
	}
	return best, true
}

// nextDisplay returns the display after current, going left to right and
// then top to bottom, and round from the last to the first.
func nextDisplay(displays []mygo.Display, current mygo.Display) (mygo.Display, bool) {
	if len(displays) == 0 {
		return mygo.Display{}, false
	}
	ordered := slices.Clone(displays)
	slices.SortStableFunc(ordered, func(a, b mygo.Display) int {
		return cmp.Or(cmp.Compare(a.Bounds.X, b.Bounds.X), cmp.Compare(a.Bounds.Y, b.Bounds.Y))
	})
	i := slices.IndexFunc(ordered, func(d mygo.Display) bool { return d.ID == current.ID })
	return ordered[(i+1)%len(ordered)], true
}
