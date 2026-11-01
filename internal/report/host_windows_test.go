package report

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"modavd/internal/win"
)

// TestPebHelper is the child of TestProcessPathAsStarted: it waits to be killed.
func TestPebHelper(t *testing.T) {
	if os.Getenv("MODAVD_REPORT_PEB_HELPER") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Minute)
}

// The path of a process is the one it was started with, as Win32_Process.ExecutablePath shows it, not the target of
// a junction on the way (a root's sdk\platform-tools is often a junction to a shared SDK).
func TestProcessPathAsStarted(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if p, err := pebImagePath(uint32(os.Getpid())); err != nil || !strings.EqualFold(p, self) {
		t.Fatalf("own path %q, %v; want %q", p, err, self)
	}

	base := testTempDir(t)
	target := filepath.Join(base, "real")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(target, "pebhelper.exe")
	in, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(exe)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(out, in)
	in.Close()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if b, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("no junction: %v %s", err, b)
	}
	started := filepath.Join(link, "pebhelper.exe")
	cmd := exec.Command(started, "-test.run=^TestPebHelper$")
	cmd.Env = append(os.Environ(), "MODAVD_REPORT_PEB_HELPER=1")
	win.HideConsoleCmd(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	h := &winHost{}
	var got []Process
	for i := 0; i < 50 && len(got) == 0; i++ {
		ps, err := h.Processes(func(n string) bool { return strings.EqualFold(n, "pebhelper.exe") })
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range ps {
			if p.PID == uint32(cmd.Process.Pid) {
				got = append(got, p)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(got) != 1 {
		t.Fatal("helper not in the process list")
	}
	if !strings.EqualFold(got[0].ExePath, started) {
		t.Errorf("path %q, want %q (as started)", got[0].ExePath, started)
	}
	if !strings.Contains(got[0].CommandLine, "-test.run=^TestPebHelper$") {
		t.Errorf("command line %q", got[0].CommandLine)
	}
}

// longPath is what [IO.Path]::GetFullPath of Windows PowerShell 5.1 gives for an existing directory: 8.3 short names
// expanded, a path without '~' as it is (case included).
func TestLongPath(t *testing.T) {
	base := testTempDir(t)
	long := filepath.Join(base, "Long Name Dir", "AndroidEmulator")
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	if p := strings.ToUpper(long); longPath(p) != p {
		t.Errorf("longPath(%q) = %q, want it unchanged", p, longPath(p))
	}
	missing := filepath.Join(base, "NOSUCH~1", "x")
	if p := longPath(missing); p != missing {
		t.Errorf("longPath(%q) = %q, want it unchanged", missing, p)
	}
	s := shortName(t, filepath.Dir(long))
	if s == "" {
		t.Skipf("no 8.3 short name for %s on this volume", filepath.Dir(long))
	}
	if p := longPath(filepath.Join(base, s, "AndroidEmulator")); p != long {
		t.Errorf("longPath of %s\\AndroidEmulator = %q, want %q", s, p, long)
	}
}
