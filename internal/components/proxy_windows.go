//go:build windows

package components

import (
	"syscall"
	"unsafe"
)

// systemProxy reads the static proxy of the current user's Internet settings (HKCU\Software\Microsoft\Windows\
// CurrentVersion\Internet Settings: ProxyEnable, ProxyServer, ProxyOverride), read only. ok=false when none is on.
func systemProxy() (server, override string, ok bool) {
	path, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	if err != nil {
		return "", "", false
	}
	var k syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_READ, &k) != nil {
		return "", "", false
	}
	defer syscall.RegCloseKey(k)
	var enable uint32
	if !regValue(k, "ProxyEnable", syscall.REG_DWORD, unsafe.Slice((*byte)(unsafe.Pointer(&enable)), 4)) || enable == 0 {
		return "", "", false
	}
	server, override = regString(k, "ProxyServer"), regString(k, "ProxyOverride")
	return server, override, server != ""
}

// regValue reads a value of type want into buf (exactly filled for a DWORD).
func regValue(k syscall.Handle, name string, want uint32, buf []byte) bool {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	var typ uint32
	n := uint32(len(buf))
	if syscall.RegQueryValueEx(k, p, nil, &typ, &buf[0], &n) != nil || typ != want {
		return false
	}
	return want != syscall.REG_DWORD || n == 4
}

// regString reads a REG_SZ value, "" when absent or of another type.
func regString(k syscall.Handle, name string) string {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return ""
	}
	var typ, n uint32
	if syscall.RegQueryValueEx(k, p, nil, &typ, nil, &n) != nil || typ != syscall.REG_SZ || n < 2 || n > 64<<10 {
		return ""
	}
	buf := make([]uint16, n/2)
	if syscall.RegQueryValueEx(k, p, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &n) != nil {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
