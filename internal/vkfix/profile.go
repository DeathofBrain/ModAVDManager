package vkfix

// Profile keeps every architecture-specific byte contract together. Libraries
// and caches from one guest ABI must never be accepted by the other patcher.
type Profile struct {
	OrigSHA256 string
	FixSHA256  string
	CacheName  string
	Patch      func([]byte) ([]byte, error)
}

func X86Profile() Profile {
	return Profile{OrigSHA256: OrigSHA256, FixSHA256: FixSHA256, CacheName: CacheName, Patch: Patch}
}

func ARM64Profile() Profile {
	return Profile{OrigSHA256: ARM64OrigSHA256, FixSHA256: ARM64FixSHA256, CacheName: ARM64CacheName, Patch: PatchARM64}
}

// ForABI selects a patch contract for the instance's guest ABI, independently
// of the manager's host architecture. Unknown ABIs must never inherit x86 code.
func ForABI(abi string) (Profile, bool) {
	switch abi {
	case "x86_64":
		return X86Profile(), true
	case "arm64-v8a":
		return ARM64Profile(), true
	default:
		return Profile{}, false
	}
}
