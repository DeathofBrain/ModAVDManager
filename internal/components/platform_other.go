//go:build !darwin || !arm64 || !cgo

package components

import (
	"os"

	"modavd/internal/root"
)

const nativeDarwinComponents = false

func (f *fetcher) platformPaths() error                    { return nil }
func (f *fetcher) platformIdle() error                     { return nil }
func (f *fetcher) platformSpace([]root.SDKComponent) error { return nil }
func moveComponent(src, dst string) error                  { return os.Rename(src, dst) }
