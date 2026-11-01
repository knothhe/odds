//go:build !darwin

package mousecenter

import "github.com/egoist/mygo"

func foregroundWindowBounds() mygo.Rectangle { return mygo.Rectangle{} }
