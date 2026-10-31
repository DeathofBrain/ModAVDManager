//go:build !windows

package components

func diskFree(string) int64 { return 0 }
