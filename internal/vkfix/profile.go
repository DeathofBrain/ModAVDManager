package vkfix

// Profile keeps a verified guest library's byte contract together. An ABI alone
// does not make a different Android release compatible with a binary patch.
type Profile struct {
	OrigSHA256 string
	FixSHA256  string
	CacheName  string
	Patch      func([]byte) ([]byte, error)
}

func X86Profile() Profile {
	return Profile{OrigSHA256: OrigSHA256, FixSHA256: FixSHA256, CacheName: CacheName, Patch: Patch}
}

// ForABI selects a patch contract for the instance's guest ABI, independently
// of the manager's host architecture. Only the API34 x86_64 patch is verified.
// The Mac API35 image has no /vendor/lib64/libvulkan_enc.so; its different Vulkan
// HAL must not inherit either the retired API34 ARM64 edits or the x86 patch.
func ForABI(abi string) (Profile, bool) {
	switch abi {
	case "x86_64":
		return X86Profile(), true
	default:
		return Profile{}, false
	}
}
