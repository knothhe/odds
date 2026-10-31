//go:build darwin

package app

import (
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"

	"github.com/knothhe/odds/source/internal/uicopy"
)

// MyGo 0.7.3 handles NotRegistered but still attempts unregister for NotFound.
// Query the documented SMAppService states before delegating real unregisters.
func clearLoginItem() error {
	if _, err := purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement", purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
		return err
	}
	class := objc.GetClass("SMAppService")
	if class == 0 {
		return fmt.Errorf(uicopy.LoginStatusFailure)
	}
	service := objc.ID(class).Send(objc.RegisterName("mainAppService"))
	status := objc.Send[int64](service, objc.RegisterName("status"))
	switch status {
	case 0, 3:
		return nil // NotRegistered / NotFound
	case 1, 2:
		return mygo.App.SetOpenAtLogin(false) // Enabled / RequiresApproval
	default:
		return fmt.Errorf(uicopy.UnknownLoginStatus, status)
	}
}
