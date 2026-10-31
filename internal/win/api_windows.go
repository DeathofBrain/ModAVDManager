//go:build windows

package win

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// HideConsoleCmd marks a child command with CREATE_NO_WINDOW.
func HideConsoleCmd(cmd *exec.Cmd) {
	if cmd != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: HideConsole}
	}
}

var procGetConsoleWindow = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")

func GetConsoleWindow() uintptr {
	if procGetConsoleWindow.Find() != nil {
		return 0
	}
	h, _, _ := procGetConsoleWindow.Call()
	return h
}

var procMessageBoxW = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

func MessageBox(msg string) error {
	text, err := syscall.UTF16PtrFromString(msg)
	if err != nil {
		return err
	}
	caption, err := syscall.UTF16PtrFromString("ModAVD 管理器")
	if err != nil {
		return err
	}
	if r, _, e := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x50010); r == 0 {
		return e
	}
	return nil
}
