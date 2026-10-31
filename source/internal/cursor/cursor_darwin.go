//go:build darwin

// Package cursor moves the mouse pointer.
package cursor

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

type cgPoint struct{ X, Y float64 }

var (
	loadOnce sync.Once
	loadErr  error
	warp     func(cgPoint) int32
	// associate reconnects the pointer to the mouse right after a warp, so
	// the pointer moves at once instead of after Quartz's short suppression.
	associate func(uint32) int32
)

func load() error {
	loadOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&warp, lib, "CGWarpMouseCursorPosition")
		purego.RegisterLibFunc(&associate, lib, "CGAssociateMouseAndMouseCursorPosition")
	})
	return loadErr
}

// MoveTo puts the pointer at x, y in global screen coordinates, in points
// from the top left of the primary display: the coordinates of
// mygo.Screen. It needs no accessibility permission.
func MoveTo(x, y float64) error {
	if err := load(); err != nil {
		return err
	}
	if code := warp(cgPoint{x, y}); code != 0 {
		return fmt.Errorf("cursor: CGWarpMouseCursorPosition failed (%d)", code)
	}
	associate(1)
	return nil
}
