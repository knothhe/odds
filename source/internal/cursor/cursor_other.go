//go:build !darwin

// Package cursor moves the mouse pointer.
package cursor

import "errors"

// MoveTo is only implemented on macOS.
func MoveTo(x, y float64) error {
	return errors.New("cursor: moving the pointer is only supported on macOS")
}
