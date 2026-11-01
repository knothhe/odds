//go:build darwin

package mousecenter

import (
	"runtime"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func TestFrontWindowBoundsSkipsOtherAppsAndOverlays(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	q := loadWindowQuery()
	if q == nil {
		t.Fatal("could not load macOS window query")
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	// Use real Foundation dictionaries and CGRect decoding with synthetic
	// metadata, so this runs without inspecting or moving the user's pointer.
	fixture := `[
		{"kCGWindowOwnerPID":8,"kCGWindowLayer":0,"kCGWindowAlpha":1,"kCGWindowBounds":{"X":10,"Y":10,"Width":500,"Height":500}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":3,"kCGWindowAlpha":1,"kCGWindowBounds":{"X":20,"Y":20,"Width":280,"Height":280}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":0,"kCGWindowAlpha":0,"kCGWindowBounds":{"X":30,"Y":30,"Width":500,"Height":500}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":0,"kCGWindowAlpha":1,"kCGWindowBounds":{}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":0,"kCGWindowAlpha":1,"kCGWindowBounds":{"X":0,"Y":0,"Width":0,"Height":500}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":0,"kCGWindowAlpha":1,"kCGWindowBounds":{"X":-1600,"Y":100,"Width":900,"Height":700}},
		{"kCGWindowOwnerPID":7,"kCGWindowLayer":0,"kCGWindowAlpha":1,"kCGWindowBounds":{"X":100,"Y":100,"Width":900,"Height":700}}
	]`
	text := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), fixture)
	data := text.Send(objc.RegisterName("dataUsingEncoding:"), uint(4)) // NSUTF8StringEncoding
	windows := objc.ID(objc.GetClass("NSJSONSerialization")).Send(objc.RegisterName("JSONObjectWithData:options:error:"), data, uint(0), uintptr(0))
	if windows == 0 {
		t.Fatal("could not decode window fixture")
	}
	want := mygo.Rectangle{X: -1600, Y: 100, Width: 900, Height: 700}
	if got := q.frontWindowBounds(windows, 7); got != want {
		t.Fatalf("frontWindowBounds = %+v; want %+v", got, want)
	}
	if got := q.frontWindowBounds(windows, 9); got != (mygo.Rectangle{}) {
		t.Fatalf("no visible window for foreground app: got %+v", got)
	}
	if got := q.frontWindowBounds(0, 7); got != (mygo.Rectangle{}) {
		t.Fatalf("no window list: got %+v", got)
	}
}
