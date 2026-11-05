//go:build !windows

// Package win is the platform boundary used by the rest of ModAVD.  The
// non-Windows implementation exposes the same API; unimplemented operations
// return errs.Unsupported. Darwin process support uses libproc through cgo.
package win

import (
	"errors"
	"os"
	"os/exec"

	"modavd/internal/errs"
)

var errUnsupported = errs.New(errs.Unsupported, "当前平台暂不支持此操作")

// Created is Unix microseconds on Darwin; identities are local to this host.
type Process struct {
	PID     uint32
	PPID    uint32
	Name    string
	Created uint64
}

type Listener struct {
	Port int
	PID  uint32
}

type StartOptions struct {
	Dir        string
	Env        []string
	Output     *os.File
	OutputPath string
}

var OrphanHelper string

const SpawnHelperArg = "__spawn"
const HideConsole = 0

type FileLock struct{ f *os.File }

// HideConsoleCmd is a no-op because non-Windows has no CREATE_NO_WINDOW flag.
func HideConsoleCmd(*exec.Cmd) {}

// SetAboveNormalPriority calls SetPriorityClass with ABOVE_NORMAL_PRIORITY_CLASS.
func SetAboveNormalPriority(Process) error { return errUnsupported }

// SetAffinity calls SetProcessAffinityMask on Windows.
func SetAffinity(Process, uint64) error { return errUnsupported }

// GetAffinity calls GetProcessAffinityMask on Windows.
func GetAffinity(Process) (uint64, error) { return 0, errUnsupported }

// L3Group is one L3 cache: its size in bytes and the logical processors sharing it.
type L3Group struct {
	Size uint32
	Mask uint64
}

// L3Groups lists the L3 caches of the host.
func L3Groups() []L3Group { return nil }

// SharedMemoryExists calls OpenFileMappingW for the Windows named mapping.
func SharedMemoryExists(string) bool { return false }

// DiskFree calls GetDiskFreeSpaceExW for the Windows volume.
func DiskFree(string) (uint64, error) { return 0, errUnsupported }

// GetCompressedFileSize calls GetCompressedFileSizeW for the Windows file.
func GetCompressedFileSize(string) (uint64, error) { return 0, errUnsupported }

// VolumeFileSystem calls GetVolumePathNameW + GetVolumeInformationW on Windows.
func VolumeFileSystem(string) (string, error) { return "", errUnsupported }

// MoveFileReplaceThrough calls MoveFileExW (REPLACE_EXISTING | WRITE_THROUGH).
func MoveFileReplaceThrough(string, string) error { return errUnsupported }

// AvailableMemoryMB calls GlobalMemoryStatusEx on Windows.
func AvailableMemoryMB() (uint64, error) { return 0, errUnsupported }

// Protect calls CryptProtectData (DPAPI) for the current Windows user.
func Protect([]byte, []byte) ([]byte, error) { return nil, errUnsupported }

// Unprotect calls CryptUnprotectData (DPAPI) for the current Windows user.
func Unprotect([]byte, []byte) ([]byte, error) { return nil, errUnsupported }

// SetEmulatorWindowsVisible finds the emulator window and calls ShowWindow on Windows.
func SetEmulatorWindowsVisible(uint32, bool) (int, error) { return 0, errUnsupported }

// GetConsoleWindow calls the Windows GetConsoleWindow API.
func GetConsoleWindow() uintptr { return 0 }

// MessageBox calls MessageBoxW from Windows user32.
func MessageBox(string) error { return errors.New("当前平台暂不支持消息框") }
