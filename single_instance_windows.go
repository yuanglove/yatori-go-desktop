//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

const desktopInstanceMutexName = "Global\\YatoriGoDesktop-9B39FBB8-08D8-4A58-917D-995E8C4E69DA"

// acquireDesktopInstance only guards the GUI process. Worker child processes
// are started before this function is called and therefore remain available for
// concurrent account tasks.
func acquireDesktopInstance() (release func(), alreadyRunning bool, err error) {
	name, err := windows.UTF16PtrFromString(desktopInstanceMutexName)
	if err != nil {
		return nil, false, fmt.Errorf("create single-instance mutex name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			if handle != 0 {
				_ = windows.CloseHandle(handle)
			}
			return func() {}, true, nil
		}
		return nil, false, fmt.Errorf("create single-instance mutex: %w", err)
	}
	return func() { _ = windows.CloseHandle(handle) }, false, nil
}
