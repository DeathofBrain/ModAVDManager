//go:build windows

package update

import (
	"os"
	"syscall"
)

const fileAttributeReparsePoint = 0x400

func isReparsePoint(st os.FileInfo) bool {
	sys, ok := st.Sys().(*syscall.Win32FileAttributeData)
	return ok && sys.FileAttributes&fileAttributeReparsePoint != 0
}
func removeLink(p string) bool {
	if os.Remove(p) == nil {
		return true
	}
	ptr, _ := syscall.UTF16PtrFromString(p)
	return syscall.RemoveDirectory(ptr) == nil
}
