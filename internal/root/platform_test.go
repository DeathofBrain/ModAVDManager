package root

import (
	"runtime"
	"testing"
)

func TestCurrentPlatformMatchesHost(t *testing.T) {
	p := CurrentPlatform()
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		if p.PackageSuffix != "win-x64" || p.ADB != "adb.exe" || p.QemuDir != "windows-x86_64" {
			t.Fatalf("Windows platform: %+v", p)
		}
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		if p.PackageSuffix != "macos-arm64" || p.ADB != "adb" {
			t.Fatalf("darwin platform: %+v", p)
		}
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		if p.PackageSuffix != "linux-x64" || p.ADB != "adb" {
			t.Fatalf("linux platform: %+v", p)
		}
	default:
		if p != (Platform{}) {
			t.Fatalf("unsupported platform inherited values: %+v", p)
		}
	}
}
