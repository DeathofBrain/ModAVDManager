package root

import "path/filepath"

// FetchComponentSpec is the fixed Windows component catalogue shared by the
// component checker and the component downloader.
type FetchComponentSpec struct {
	ID             string
	Name           string
	Version        string
	URL            string
	Size           int64
	SHA1           string
	Target         string
	ArchiveRoot    string
	KeyFiles       []string
	ExtractedBytes int64
	KeyHashes      map[string]string
	Properties     string
}

// WindowsFetchComponents returns a copy of the Google components used by the
// Windows release. Darwin and Linux intentionally have no catalogue yet.
func WindowsFetchComponents() []FetchComponentSpec {
	return []FetchComponentSpec{
		{
			ID: "platform-tools", Name: PlatformToolsName, Version: "37.0.1",
			URL: "platform-tools_r37.0.1-win.zip", Size: 8044989,
			SHA1:   "e03e78b1d80b396f1c3358e31251cb31740e1110",
			Target: "platform-tools", ArchiveRoot: "platform-tools",
			KeyFiles: []string{"adb.exe", "AdbWinApi.dll", "AdbWinUsbApi.dll"}, ExtractedBytes: 20 << 20,
			KeyHashes: map[string]string{
				"adb.exe":          "b4a6b455702684652cccf7b46258b29e653538904359a58fd4931cf3ef286b3f",
				"AdbWinApi.dll":    "c1d653030b4bde65d3e07e4d0b0979e17be56df1436cdd15528630f27808050d",
				"AdbWinUsbApi.dll": "0710e894d9b40f71a670c13c694079d564c92c1279da382cfe4850983aaebe1b",
			},
			Properties: "Pkg.UserSrc=false\nPkg.Revision=37.0.1\n",
		},
		{
			ID: "system-image", Name: SystemImageName, Version: "14",
			URL: "sys-img/google_apis/x86_64-34_r14.zip", Size: 1563721130,
			SHA1:   "e0f6c9a0691aa27bd597d0deb1bcfdc943ac8ca7",
			Target: filepath.FromSlash("system-images/android-34/google_apis/x86_64"), ArchiveRoot: "x86_64",
			KeyFiles: []string{"system.img", "vendor.img", "kernel-ranchu", "ramdisk.img"}, ExtractedBytes: 4500 << 20,
			Properties: "Pkg.Desc=System Image x86_64 with Google APIs.\nPkg.Revision=14\nPkg.Dependencies=emulator#34.2.16\nAndroidVersion.ApiLevel=34\nAndroidVersion.ExtensionLevel=7\nAndroidVersion.IsBaseSdk=true\nSystemImage.Abi=x86_64\nSystemImage.TagId=google_apis\nSystemImage.TagDisplay=Google APIs\nSystemImage.GpuSupport=true\nAddon.VendorId=google\nAddon.VendorDisplay=Google Inc.\n",
		},
	}
}
