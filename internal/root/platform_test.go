package root

import (
	"runtime"
	"testing"
)

func TestCurrentPlatformMatchesHost(t *testing.T) {
	p := CurrentPlatform()
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		want := Platform{Emulator: "emulator.exe", ADB: "adb.exe", QEMU: "qemu-system-x86_64.exe", QEMUHeadless: "qemu-system-x86_64-headless.exe", QemuDir: "windows-x86_64", PackageSuffix: "win-x64", MirrorOS: "win", MirrorArch: "x64"}
		if p != want {
			t.Fatalf("Windows platform: %+v", p)
		}
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		want := Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-aarch64", QEMUHeadless: "qemu-system-aarch64", PackageSuffix: "macos-arm64", MirrorOS: "macos", MirrorArch: "arm64"}
		if p != want {
			t.Fatalf("darwin platform: %+v", p)
		}
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		want := Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-x86_64", QEMUHeadless: "qemu-system-x86_64", PackageSuffix: "linux-x64", MirrorOS: "linux", MirrorArch: "x64"}
		if p != want {
			t.Fatalf("linux platform: %+v", p)
		}
	default:
		if p != (Platform{}) {
			t.Fatalf("unsupported platform inherited values: %+v", p)
		}
	}
}
