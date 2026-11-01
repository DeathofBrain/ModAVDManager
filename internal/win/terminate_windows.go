//go:build windows

package win

import "syscall"

func TerminateRefused(p Process) bool {
	if p.PID == 0 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, p.PID)
	if err == nil {
		syscall.CloseHandle(h)
		return false
	}
	if err == syscall.Errno(87) {
		return false
	}
	if p.Created != 0 {
		if h, err := syscall.OpenProcess(0x1000, false, p.PID); err == nil {
			defer syscall.CloseHandle(h)
			var c, e, k, u syscall.Filetime
			if syscall.GetProcessTimes(h, &c, &e, &k, &u) == nil && uint64(c.HighDateTime)<<32|uint64(c.LowDateTime) != p.Created {
				return false
			}
		}
	}
	return true
}
