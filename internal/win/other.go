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

func HideConsoleCmd(*exec.Cmd) {}

func ListProcesses() ([]Process, error)          { return nil, errUnsupported }
func FindProcess(uint32) (Process, bool)         { return Process{}, false }
func ProcessName(uint32) string                  { return "" }
func (p Process) CommandLine() ([]string, error) { return nil, errUnsupported }
func (p Process) ImagePath() (string, error)     { return "", errUnsupported }
func (p Process) StartedAt() time.Time {
	if p.Created == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(p.Created)*100)
}
func (p Process) Alive() bool                    { return false }
func (p Process) Wait(time.Duration) bool        { return true }
func (p Process) ExitCode() (uint32, bool)       { return 0, false }
func (p Process) CPUTime() (time.Duration, bool) { return 0, false }
func (p Process) Kill() error                    { return errUnsupported }
func KillTree(Process) error                     { return errUnsupported }

func StartDetached(string, []string, StartOptions) (Process, error) { return Process{}, errUnsupported }
func StartOrphan(string, []string, StartOptions) (Process, error)   { return Process{}, errUnsupported }
func SpawnHelperMain([]string, *os.File) int                        { return 1 }

func TryLock(string) (*FileLock, bool, error)             { return nil, false, errUnsupported }
func Lock(string, time.Duration) (*FileLock, bool, error) { return nil, false, errUnsupported }
func (l *FileLock) Unlock()                               {}

func Listeners() ([]Listener, error)                              { return nil, errUnsupported }
func ListenerPID(int) (uint32, bool, error)                       { return 0, false, errUnsupported }
func BusyPorts([]int) ([]int, error)                              { return nil, errUnsupported }
func SetAboveNormalPriority(Process) error                        { return errUnsupported }
func SetAffinity(Process, uint64) error                           { return errUnsupported }
func BigL3CPUs() (uint64, []int, bool)                            { return 0, nil, false }
func SharedMemoryExists(string) bool                              { return false }
func DiskFree(string) (uint64, error)                             { return 0, errUnsupported }
func AvailableMemoryMB() (uint64, error)                          { return 0, errUnsupported }
func Protect([]byte, []byte) ([]byte, error)                      { return nil, errUnsupported }
func Unprotect([]byte, []byte) ([]byte, error)                    { return nil, errUnsupported }
func SetEmulatorWindowsVisible(uint32, string, bool) (int, error) { return 0, errUnsupported }
func GetConsoleWindow() uintptr                                   { return 0 }
func MessageBox(string) error                                     { return errors.New("当前平台暂不支持消息框") }
