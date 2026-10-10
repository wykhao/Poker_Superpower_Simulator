//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"os"
	"strings"
	"unsafe"
)

func showStartupError(err error) {
	for _, arg := range os.Args {
		if strings.Contains(arg, "no-browser") {
			return
		}
	}
	text, _ := windows.UTF16PtrFromString(err.Error())
	title, _ := windows.UTF16PtrFromString("Poker Superpower Simulator")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
