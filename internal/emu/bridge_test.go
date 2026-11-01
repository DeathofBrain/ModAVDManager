//go:build windows

package emu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modavd/internal/errs"
	"modavd/internal/win"
)

func TestBridgeScriptRejectsRelativeSystemRoot(t *testing.T) {
	m := New(managerTestRoot(t, 1), nil)
	if err := os.MkdirAll(filepath.Join(m.Root.Dir, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Root.Dir, "tools", bridgeScriptName), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, systemRoot := range []string{"relative", ""} {
		t.Run(systemRoot, func(t *testing.T) {
			t.Setenv("SystemRoot", systemRoot)
			run, err := m.runBridgeScript(context.Background(), 1, "verify", false)
			if errs.CodeOf(err) != errs.Internal {
				t.Fatalf("SystemRoot=%q: error=%v, want internal", systemRoot, err)
			}
			if run.Code != -1 {
				t.Fatalf("SystemRoot=%q: run=%+v, relative PowerShell may have run", systemRoot, run)
			}
		})
	}
}

// TestBridgeLockExcludes: while some process holds an instance's bridge lock (an install, with its stops and cold
// boots), status says busy "bridging", a second bridge action and a delete are refused as busy, and the delete that
// runs afterwards removes the lock file with the others.
func TestBridgeLockExcludes(t *testing.T) {
	m, _ := opsTestManager(t)
	ctx := context.Background()
	i := 6
	_, err := m.Create(ctx, CreateRequest{filesOnly: true, Index: &i})
	opsSkipIfNotImplemented(t, err)
	if err != nil {
		t.Fatal(err)
	}
	l, ok, err := win.TryLock(m.lockPath(i, "bridge"))
	if err != nil || !ok {
		t.Fatalf("TryLock: %v %v", ok, err)
	}
	info, err := m.Info(ctx, i)
	if err != nil || info.Busy != "bridging" {
		l.Unlock()
		t.Fatalf("busy = %q, %v", info.Busy, err)
	}
	for _, action := range []string{"install", "uninstall", "verify"} {
		if _, err := m.Bridge(ctx, i, action); errs.CodeOf(err) != errs.Busy {
			l.Unlock()
			t.Fatalf("bridge %s while bridging: %v", action, err)
		}
	}
	if _, err := m.Delete(ctx, i); errs.CodeOf(err) != errs.Busy || !strings.Contains(err.Error(), "正在装卸执行桥") {
		l.Unlock()
		t.Fatalf("delete while bridging: %v", err)
	}
	l.Unlock()

	if _, err := m.Delete(ctx, i); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"start", "stop", "bridge"} {
		if fileOrDirExists(m.lockPath(i, kind)) {
			t.Errorf("%s lock file left after delete", kind)
		}
	}
}
