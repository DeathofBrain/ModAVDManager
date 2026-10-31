package root

import "runtime"

func platformKeyFiles() map[string][]string {
	p := CurrentPlatform()
	qemu := p.QEMU
	if p.QemuDir != "" {
		qemu = "qemu/" + p.QemuDir + "/" + p.QEMU
	}
	if p.QemuDir == "" {
		qemu = "qemu/" + p.QEMU
	}
	return map[string][]string{
		"platform-tools": {p.ADB},
		"emulator":       {p.Emulator, qemu},
		"system-image":   {"system.img", "vendor.img", "kernel-ranchu", "ramdisk.img"},
	}
}

// Platform describes host-specific executable names and update identifiers.
// Callers use this table instead of embedding Windows names in shared logic.
type Platform struct {
	Emulator      string
	ADB           string
	QEMU          string
	QEMUHeadless  string
	QemuDir       string
	PackageSuffix string
	MirrorOS      string
	MirrorArch    string
}

func CurrentPlatform() Platform {
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		// The qemu directory on macOS is expected to be qemu/darwin-aarch64;
		// #8 will verify the package layout before changing this table.
		return Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-aarch64", QEMUHeadless: "qemu-system-aarch64", PackageSuffix: "macos-arm64", MirrorOS: "macos", MirrorArch: "arm64"}
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return Platform{Emulator: "emulator", ADB: "adb", QEMU: "qemu-system-x86_64", QEMUHeadless: "qemu-system-x86_64", PackageSuffix: "linux-x64", MirrorOS: "linux", MirrorArch: "x64"}
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return Platform{Emulator: "emulator.exe", ADB: "adb.exe", QEMU: "qemu-system-x86_64.exe", QEMUHeadless: "qemu-system-x86_64-headless.exe", QemuDir: "windows-x86_64", PackageSuffix: "win-x64", MirrorOS: "win", MirrorArch: "x64"}
	default:
		// An unlisted host must not inherit Windows names or MirrorChyan ids.
		return Platform{}
	}
}
