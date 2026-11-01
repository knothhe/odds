//go:build darwin

package mousecenter

import (
	"log"
	"math"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

type cgWindowRect struct{ X, Y, Width, Height float64 }

type windowQuery struct {
	list   func(uint32, uint32) objc.ID
	bounds func(objc.ID, *cgWindowRect) bool
}

var loadWindowQuery = sync.OnceValue(func() *windowQuery {
	if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
		log.Printf("mouse-center: %v", err)
		return nil
	}
	lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		log.Printf("mouse-center: %v", err)
		return nil
	}
	q := &windowQuery{}
	purego.RegisterLibFunc(&q.list, lib, "CGWindowListCopyWindowInfo")
	purego.RegisterLibFunc(&q.bounds, lib, "CGRectMakeWithDictionaryRepresentation")
	return q
})

// foregroundWindowBounds reads only public window metadata; it needs neither
// Accessibility nor Screen Recording permission. MyGo's FocusedWindow only
// covers our own app, so query the frontmost application's visible windows.
// Call on the UI thread, as with the tool's menu and global-shortcut callbacks.
func foregroundWindowBounds() mygo.Rectangle {
	q := loadWindowQuery()
	if q == nil {
		return mygo.Rectangle{}
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(objc.RegisterName("sharedWorkspace"))
	front := workspace.Send(objc.RegisterName("frontmostApplication"))
	pid := objc.Send[int32](front, objc.RegisterName("processIdentifier"))
	if pid <= 0 {
		return mygo.Rectangle{}
	}
	// kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements.
	windows := q.list(1|16, 0)
	if windows == 0 {
		return mygo.Rectangle{}
	}
	// The copied CFArray is toll-free bridged to NSArray and owned by us.
	defer windows.Send(objc.RegisterName("release"))
	return q.frontWindowBounds(windows, pid)
}

func (q *windowQuery) frontWindowBounds(windows objc.ID, pid int32) mygo.Rectangle {
	value := func(dictionary objc.ID, key string) objc.ID {
		name := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), key)
		return dictionary.Send(objc.RegisterName("objectForKey:"), name)
	}
	// Quartz returns windows front to back. Ignore other apps and floating
	// panels (including our nonactivating highlight) to find the working window.
	count := objc.Send[uint](windows, objc.RegisterName("count"))
	for i := uint(0); i < count; i++ {
		window := windows.Send(objc.RegisterName("objectAtIndex:"), i)
		if objc.Send[int32](value(window, "kCGWindowOwnerPID"), objc.RegisterName("intValue")) != pid ||
			objc.Send[int32](value(window, "kCGWindowLayer"), objc.RegisterName("intValue")) != 0 ||
			objc.Send[float64](value(window, "kCGWindowAlpha"), objc.RegisterName("doubleValue")) <= 0 {
			continue
		}
		var r cgWindowRect
		if !q.bounds(value(window, "kCGWindowBounds"), &r) || r.Width <= 0 || r.Height <= 0 {
			continue
		}
		// Quartz already uses MyGo's top-left primary-display origin in points,
		// including negative coordinates; no Retina scaling or Y flip is needed.
		return mygo.Rectangle{
			X: int(math.Round(r.X)), Y: int(math.Round(r.Y)),
			Width: int(math.Round(r.Width)), Height: int(math.Round(r.Height)),
		}
	}
	return mygo.Rectangle{}
}
