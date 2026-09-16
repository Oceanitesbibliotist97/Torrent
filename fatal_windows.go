//go:build windows

package main

import "golang.org/x/sys/windows"

// showFatal reports a startup failure; a GUI app has no console to print to.
func showFatal(title, msg string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
