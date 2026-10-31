//go:build !darwin

package app

import (
	"fmt"

	"github.com/knothhe/odds/source/internal/uicopy"
)

func clearLoginItem() error { return fmt.Errorf("%s", uicopy.CleanupMacOnly()) }
