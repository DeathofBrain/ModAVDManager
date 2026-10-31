//go:build windows

package report

import (
	"os/exec"
)

func copyClipboard(text string) error {
	// clip.exe is part of Windows and consumes UTF-16/UTF-8 text from stdin.
	// It does not start an adb server and exits immediately.
	p := exec.Command("clip.exe")
	in, err := p.StdinPipe()
	if err != nil {
		return err
	}
	if err := p.Start(); err != nil {
		return err
	}
	if _, err := in.Write([]byte(text)); err != nil {
		_ = in.Close()
		_ = p.Wait()
		return err
	}
	_ = in.Close()
	return p.Wait()
}

func openExplorer(path string) error { return exec.Command("explorer.exe", "/select,"+path).Run() }
