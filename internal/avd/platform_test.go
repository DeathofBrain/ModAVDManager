package avd

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"modavd/internal/root"
)

func TestCreateHostGuestProfile(t *testing.T) {
	var abi, arch, imageDir string
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		abi, arch, imageDir = "x86_64", "x86_64", `system-images\android-34\google_apis\x86_64`
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		abi, arch, imageDir = "arm64-v8a", "arm64", "system-images/android-34/google_apis/arm64-v8a"
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		abi, arch, imageDir = "x86_64", "x86_64", "system-images/android-34/google_apis/x86_64"
	default:
		t.Skip("no guest profile for this host")
	}
	for _, localSDK := range []bool{false, true} {
		t.Run(map[bool]string{false: "packaged", true: "sdkRoot"}[localSDK], func(t *testing.T) {
			r, err := root.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if localSDK {
				if err := os.WriteFile(r.MetadataPath(), []byte(`{"sdkRoot":"runtime sdk"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			imagePath := filepath.Join(r.RuntimeSDK(), imageDir, "system.img")
			if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(imagePath, []byte("host image fixture"), 0o644); err != nil {
				t.Fatal(err)
			}
			in, err := Create(r, 1, CreateOptions{Title: "平台配置", MemoryMB: 4096, CPU: 4, DataPartitionGB: 16, FPS: 60})
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := ReadIni(ConfigPath(r, in.Index))
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{
				"abi.type": abi, "hw.cpu.arch": arch,
				"image.sysdir.1": imageDir + string(filepath.Separator),
				"hw.cpu.ncore":   "4", "hw.ramSize": "4096M", "avd.ini.displayname": "平台配置",
			} {
				if got := cfg.Value(key); got != want {
					t.Errorf("%s = %q; want %q", key, got, want)
				}
			}
			// Resolve the actual written image.sysdir.1 against the selected SDK, including sdkRoot overrides.
			image, err := os.ReadFile(filepath.Join(r.RuntimeSDK(), cfg.Value("image.sysdir.1"), "system.img"))
			if err != nil || string(image) != "host image fixture" {
				t.Fatalf("written system-image path does not resolve: %q, %v", image, err)
			}
			ini, err := ReadIni(IniPath(r, in.Index))
			if err != nil {
				t.Fatal(err)
			}
			if ini.Value("path") != in.Dir || ini.Value("path.rel") != filepath.Join("avd", "mas_1.avd") {
				t.Fatalf("AVD directory paths: path=%q path.rel=%q", ini.Value("path"), ini.Value("path.rel"))
			}
			if _, err := os.Stat(filepath.Join(r.Dir, ini.Value("path.rel"), "config.ini")); err != nil {
				t.Fatalf("written path.rel does not resolve to the AVD: %v", err)
			}
			if !slices.Equal(ini.Keys(), []string{"avd.ini.encoding", "path", "path.rel", "target"}) {
				t.Fatalf("AVD .ini key order changed: %v", ini.Keys())
			}
			keys := cfg.Keys()
			if len(keys) < len(ConfigTemplate) {
				t.Fatalf("config has fewer keys than its template: %v", keys)
			}
			for n, kv := range ConfigTemplate {
				if keys[n] != kv[0] {
					t.Fatalf("template key %d moved: got %s; want %s", n, keys[n], kv[0])
				}
			}
		})
	}
}
