//go:build !windows

// Package win is the platform boundary used by the rest of ModAVD.  The
// non-Windows implementation deliberately exposes the same API while
// returning errs.Unsupported for operations that need a host implementation.
package win

import (
	"errors"
	"os"
	"os/exec"
	"time"

	"modavd/internal/errs"
)

var errUnsupported = errs.New(errs.Unsupported, "当前平台暂不支持此操作")

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

type FileLock struct{}

// HideConsoleCmd is a no-op because non-Windows has no CREATE_NO_WINDOW flag.
func HideConsoleCmd(*exec.Cmd) {}

// ListProcesses cannot inspect the Windows process table on this host.
func ListProcesses() ([]Process, error) { return nil, errUnsupported }

// FindProcess cannot resolve Windows process ids on this host.
func FindProcess(uint32) (Process, bool) { return Process{}, false }

// ProcessName has no Windows process table to query on this host.
func ProcessName(uint32) string { return "" }

// CommandLine is unavailable without the Windows process APIs.
func (p Process) CommandLine() ([]string, error) { return nil, errUnsupported }

// ImagePath is unavailable without the Windows process APIs.
func (p Process) ImagePath() (string, error) { return "", errUnsupported }

// StartedAt converts the Windows FILETIME value when a caller supplies one.
func (p Process) StartedAt() time.Time {
	if p.Created == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(p.Created)*100)
}

// Alive cannot inspect a Windows process on this host.
func (p Process) Alive() bool { return false }

// Wait has nothing to wait for when the process cannot be observed.
func (p Process) Wait(time.Duration) bool { return true }

// ExitCode cannot inspect a Windows process on this host.
func (p Process) ExitCode() (uint32, bool) { return 0, false }

// CPUTime cannot inspect a Windows process on this host.
func (p Process) CPUTime() (time.Duration, bool) { return 0, false }

// Kill cannot terminate a Windows process on this host.
func (p Process) Kill() error { return errUnsupported }

// KillTree cannot terminate a Windows process tree on this host.
func KillTree(Process) error { return errUnsupported }

// StartDetached requires the Windows detached process implementation.
func StartDetached(string, []string, StartOptions) (Process, error) { return Process{}, errUnsupported }

// StartOrphan requires the Windows orphan helper implementation.
func StartOrphan(string, []string, StartOptions) (Process, error) { return Process{}, errUnsupported }

// SpawnHelperMain is only meaningful for the Windows helper process.
func SpawnHelperMain([]string, *os.File) int { return 1 }

// TryLock requires Windows LockFileEx semantics.
func TryLock(string) (*FileLock, bool, error) { return nil, false, errUnsupported }

// Lock requires Windows LockFileEx semantics.
func Lock(string, time.Duration) (*FileLock, bool, error) { return nil, false, errUnsupported }

// Unlock is a no-op for the unavailable lock.
func (l *FileLock) Unlock() {}

// Listeners cannot inspect the Windows listener table on this host.
func Listeners() ([]Listener, error) { return nil, errUnsupported }

// ListenerPID cannot inspect the Windows listener table on this host.
func ListenerPID(int) (uint32, bool, error) { return 0, false, errUnsupported }

// BusyPorts cannot inspect the Windows listener table on this host.
func BusyPorts([]int) ([]int, error) { return nil, errUnsupported }

// SetAboveNormalPriority requires Windows process handles.
func SetAboveNormalPriority(Process) error { return errUnsupported }

// SetAffinity requires Windows process handles.
func SetAffinity(Process, uint64) error { return errUnsupported }

// BigL3CPUs requires Windows CPU topology APIs.
func BigL3CPUs() (uint64, []int, bool) { return 0, nil, false }

// SharedMemoryExists checks a Windows named mapping; it is unavailable here.
func SharedMemoryExists(string) bool { return false }

// DiskFree requires Windows volume APIs.
func DiskFree(string) (uint64, error) { return 0, errUnsupported }

// AvailableMemoryMB requires Windows memory APIs.
func AvailableMemoryMB() (uint64, error) { return 0, errUnsupported }

// Protect requires Windows DPAPI.
func Protect([]byte, []byte) ([]byte, error) { return nil, errUnsupported }

// Unprotect requires Windows DPAPI.
func Unprotect([]byte, []byte) ([]byte, error) { return nil, errUnsupported }

// SetEmulatorWindowsVisible requires Windows window APIs.
func SetEmulatorWindowsVisible(uint32, string, bool) (int, error) { return 0, errUnsupported }

// GetConsoleWindow has no Windows console to query here.
func GetConsoleWindow() uintptr { return 0 }

// MessageBox requires the Windows user32 message box API.
func MessageBox(string) error { return errors.New("当前平台暂不支持消息框") }
