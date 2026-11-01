//go:build !windows

package update

import "os"

func isReparsePoint(st os.FileInfo) bool { return st.Mode()&os.ModeSymlink != 0 }
func removeLink(p string) bool           { return os.Remove(p) == nil }
