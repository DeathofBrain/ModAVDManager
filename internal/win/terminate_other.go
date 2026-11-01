//go:build !windows

package win

func TerminateRefused(Process) bool { return false }
