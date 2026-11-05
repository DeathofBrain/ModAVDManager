package root

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCurrentPlatformMatchesHost(t *testing.T) {
	p := CurrentPlatform()
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		want := Platform{Emulator: "emulator.exe", ADB: "adb.exe", QEMU: "qemu-system-x86_64.exe", QEMUHeadless: "qemu-system-x86_64-headless.exe", QemuDir: "windows-x86_64", PackageSuffix: "win-x64", MirrorOS: "win", MirrorArch: "x64", GuestABI: "x86_64", GuestArch: "x86_64", SystemImageDir: `system-images\android-34\google_apis\x86_64`, SystemImageName: "系统镜像 Android 14 Google APIs x86_64"}
		if p != want {
			t.Fatalf("Windows platform: %+v", p)
		}
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		want := Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-aarch64", QEMUHeadless: "qemu-system-aarch64-headless", QemuDir: "darwin-aarch64", PackageSuffix: "macos-arm64", MirrorOS: "macos", MirrorArch: "arm64", GuestABI: "arm64-v8a", GuestArch: "arm64", SystemImageDir: "system-images/android-34/google_apis/arm64-v8a", SystemImageName: "系统镜像 Android 14 Google APIs arm64-v8a"}
		if p != want {
			t.Fatalf("darwin platform: %+v", p)
		}
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		want := Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-x86_64", QEMUHeadless: "qemu-system-x86_64", PackageSuffix: "linux-x64", MirrorOS: "linux", MirrorArch: "x64", GuestABI: "x86_64", GuestArch: "x86_64", SystemImageDir: "system-images/android-34/google_apis/x86_64", SystemImageName: "系统镜像 Android 14 Google APIs x86_64"}
		if p != want {
			t.Fatalf("linux platform: %+v", p)
		}
	default:
		if p != (Platform{}) {
			t.Fatalf("unsupported platform inherited values: %+v", p)
		}
	}
	wantSupported := runtime.GOOS == "windows" && runtime.GOARCH == "amd64" ||
		runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" ||
		runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
	if p.Supported() != wantSupported || p.Windows() != (runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Fatalf("host %s/%s has incorrect platform capabilities: %+v", runtime.GOOS, runtime.GOARCH, p)
	}
}

func TestWindowsImageConstantsStayCompatible(t *testing.T) {
	// WindowsComponents is also inspected on Unix hosts; these constants must not follow the host profile.
	if SystemImageDir != `system-images\android-34\google_apis\x86_64` ||
		SystemImageName != "系统镜像 Android 14 Google APIs x86_64" {
		t.Fatalf("Windows image constants changed: %q, %q", SystemImageDir, SystemImageName)
	}
	if (Platform{}).Supported() || (Platform{}).Windows() {
		t.Fatal("an unlisted platform must not inherit a supported host")
	}
}

func TestPlatformRuntimeSDKPaths(t *testing.T) {
	p := CurrentPlatform()
	if !p.Supported() {
		t.Skip("no executable layout for this host")
	}
	for _, localSDK := range []bool{false, true} {
		t.Run(map[bool]string{false: "packaged", true: "sdkRoot"}[localSDK], func(t *testing.T) {
			r, err := New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			sdk := filepath.Join(r.Dir, "sdk")
			if localSDK {
				if err := os.WriteFile(r.MetadataPath(), []byte(`{"sdkRoot":"runtime sdk"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				sdk = filepath.Join(r.Dir, "runtime sdk")
			}
			if r.RuntimeSDK() != sdk || r.EmulatorExe() != filepath.Join(sdk, "emulator", p.Emulator) ||
				r.AdbExe() != filepath.Join(sdk, "platform-tools", p.ADB) ||
				r.QemuHeadlessExe() != filepath.Join(sdk, "emulator", "qemu", p.QemuDir, p.QEMUHeadless) {
				t.Fatalf("runtime SDK layout: sdk=%q emulator=%q adb=%q qemu=%q", r.RuntimeSDK(), r.EmulatorExe(), r.AdbExe(), r.QemuHeadlessExe())
			}
			qemu := filepath.Join(sdk, "emulator", "qemu", p.QemuDir, p.QEMU)
			for _, path := range []string{r.EmulatorExe(), qemu} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if !r.EmulatorPresent() {
				t.Fatal("component detection did not find the host's emulator layout")
			}
			if err := os.Remove(qemu); err != nil {
				t.Fatal(err)
			}
			if r.EmulatorPresent() {
				t.Fatal("emulator reported present without its host QEMU binary")
			}
		})
	}
}
