//go:build windows

package main

import "syscall"

func init() {
	// Force Windows console codepage to UTF-8 (65001) so worker stdout
	// is never re-encoded to GBK by the Windows runtime.
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	kernel32.NewProc("SetConsoleOutputCP").Call(65001)
}
