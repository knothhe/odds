//go:build !darwin

package app

import "fmt"

func clearLoginItem() error { return fmt.Errorf("Dev 安装清理仅支持 macOS") }
