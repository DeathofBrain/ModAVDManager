package emu

import (
	"runtime"

	"modavd/internal/errs"
)

// requireWindows gates operations whose host implementation is still Windows-only.
// Keeping this check at the public operation boundary prevents a non-Windows stub
// from creating markers, metadata, locks, or other partial state before failing.
func requireWindows() error {
	if runtime.GOOS != "windows" {
		return errs.New(errs.Unsupported, "当前平台暂不支持此操作")
	}
	return nil
}
