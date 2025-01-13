//go:build windows

package terminal

import (
	"golang.org/x/sys/windows"
)

// HideConsole hides the console window and activates another window
func HideConsole() {
	getConsoleWindow := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
	showWindow := windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow")
	if getConsoleWindow.Find() == nil && showWindow.Find() == nil {
		hwnd, _, _ := getConsoleWindow.Call()
		if hwnd != 0 {
			_, _, _ = showWindow.Call(hwnd, 0)
		}
	}
}
